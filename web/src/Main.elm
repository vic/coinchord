port module Main exposing (main)

import Api exposing (..)
import Browser
import Dict exposing (Dict)
import Draw
import Html exposing (..)
import Html.Attributes as A exposing (class, classList, href, placeholder, style, title, type_, value)
import Html.Events exposing (on, onBlur, onClick, onInput)
import Html.Lazy exposing (lazy7)
import Json.Decode as D
import Json.Encode as E
import Svg
import Svg.Attributes as SA
import Time


port send : E.Value -> Cmd msg


port receive : (D.Value -> msg) -> Sub msg



-- MODEL


type alias Ui =
    { setupName : String
    , layerName : String
    , bindText : String
    , editing : Maybe String
    , renames : Dict Int String
    , drag : Maybe Float
    , knobs : Dict Int Float
    , tab : String
    , picked : Maybe String
    , confirmReset : Bool
    }


type alias Model =
    { page : String
    , st : Maybe State
    , frame : Frame
    , history : List Float
    , stroke : Maybe Stroke
    , flashG : Dict String Float
    , flashS : Dict String Float
    , now : Float
    , connected : Bool
    , error : String
    , ui : Ui
    }


init : String -> ( Model, Cmd Msg )
init hash =
    ( { page = ifThen (List.any (\( p, _, _ ) -> p == hash) pages) hash "kits"
      , st = Nothing
      , frame = Frame 0 0 0 0 []
      , history = []
      , stroke = Nothing
      , flashG = Dict.empty
      , flashS = Dict.empty
      , now = 0
      , connected = False
      , error = ""
      , ui = Ui "" "" "" Nothing Dict.empty Nothing Dict.empty "detection" Nothing False
      }
    , Cmd.none
    )


historyLength : Int
historyLength =
    240


flashMs : Float
flashMs =
    900



-- UPDATE


type Msg
    = Got D.Value
    | Go String
    | Do String (List ( String, E.Value ))
    | DoRaw (List D.Value)
    | Tick Time.Posix
    | SetUi (Ui -> Ui)
    | DragAt Float
    | DragEnd


cmd : String -> List ( String, E.Value ) -> Cmd msg
cmd name fields =
    send (E.object (( "do", E.string name ) :: fields))


int : String -> Int -> ( String, E.Value )
int k v =
    ( k, E.int v )


str : String -> String -> ( String, E.Value )
str k v =
    ( k, E.string v )


num : String -> Float -> ( String, E.Value )
num k v =
    ( k, E.float v )


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        Got v ->
            received v model

        Go p ->
            ( { model | page = p }, cmd "page" [ str "name" p ] )

        Do name fields ->
            ( model, cmd name fields )

        DoRaw cmds ->
            let
                pageOf c =
                    case ( D.decodeValue (D.field "do" D.string) c, D.decodeValue (D.field "name" D.string) c ) of
                        ( Ok "page", Ok p ) ->
                            Just p

                        _ ->
                            Nothing

                page =
                    List.filterMap pageOf cmds |> List.head |> Maybe.withDefault model.page
            in
            ( { model | page = page }, Cmd.batch (List.map send cmds) )

        Tick t ->
            ( { model | now = toFloat (Time.posixToMillis t) }, Cmd.none )

        SetUi f ->
            ( { model | ui = f model.ui }, Cmd.none )

        DragAt l ->
            let
                ui =
                    model.ui
            in
            ( { model | ui = { ui | drag = Just (clamp 0.01 0.99 l) } }, Cmd.none )

        DragEnd ->
            case ( model.ui.drag, model.st ) of
                ( Just l, Just st ) ->
                    let
                        ui =
                            model.ui
                    in
                    ( { model | ui = { ui | drag = Nothing } }
                    , case knobIndex "Trigger level" st of
                        Just i ->
                            cmd "knob" [ int "i" i, num "value" (rmsOf l) ]

                        Nothing ->
                            Cmd.none
                    )

                _ ->
                    ( model, Cmd.none )


received : D.Value -> Model -> ( Model, Cmd Msg )
received v model =
    case D.decodeValue (D.field "t" D.string) v of
        Ok "frame" ->
            case D.decodeValue frame v of
                Ok f ->
                    ( { model | frame = f, history = List.take historyLength (f.level :: model.history) }, Cmd.none )

                Err _ ->
                    ( model, Cmd.none )

        Ok "stroke" ->
            case D.decodeValue stroke v of
                Ok s ->
                    ( { model
                        | stroke = Just s
                        , flashG = Dict.insert s.gesture.name model.now model.flashG
                        , flashS = List.foldl (\code -> Dict.insert code model.now) model.flashS (String.split "&" s.gesture.src)
                      }
                    , Cmd.none
                    )

                Err _ ->
                    ( model, Cmd.none )

        Ok "state" ->
            case D.decodeValue state v of
                Ok s ->
                    ( { model | st = Just s, error = "" }, Cmd.none )

                Err e ->
                    ( { model | error = D.errorToString e }, Cmd.none )

        Ok "open" ->
            ( { model | connected = True }, cmd "page" [ str "name" model.page ] )

        Ok "closed" ->
            ( { model | connected = False }, Cmd.none )

        _ ->
            ( model, Cmd.none )


knobIndex : String -> State -> Maybe Int
knobIndex name st =
    st.knobs
        |> List.indexedMap Tuple.pair
        |> List.filter (\( _, k ) -> k.name == name)
        |> List.head
        |> Maybe.map Tuple.first


knobValue : String -> State -> Float
knobValue name st =
    st.knobs |> List.filter (\k -> k.name == name) |> List.head |> Maybe.map .value |> Maybe.withDefault 0


{-| level is 0..1 on a -90..0 dB scale; rms is linear amplitude. -}
rmsOf : Float -> Float
rmsOf l =
    10 ^ ((l * 90 - 90) / 20)


levelOf : Float -> Float
levelOf rms =
    clamp 0 1 ((20 * logBase 10 (max rms 0.00001) + 90) / 90)



-- VIEW


pages : List ( String, String, String )
pages =
    [ ( "kits", "🪙", "Kits" )
    , ( "calibrate", "🎯", "Calibrate" )
    , ( "gestures", "✋", "Gestures" )
    , ( "tutor", "🎓", "Tutor" )
    , ( "layers", "🗂", "Layers" )
    , ( "settings", "⚙️", "Settings" )
    ]


view : Model -> Html Msg
view model =
    case model.st of
        Nothing ->
            div [ class "loading" ]
                [ div [ class "logo big" ] [ text "◎" ]
                , h2 [] [ text "CoinChord" ]
                , p [] [ text (ifThen model.connected "Loading…" "Connecting to CoinChord…") ]
                , ifThen (model.error /= "") (pre [ class "error" ] [ text model.error ]) (text "")
                ]

        Just st ->
            div [ class "app" ]
                [ sidebar model st
                , div [ class "main" ]
                    [ topbar st
                    , liveStrip model st
                    , notice st
                    , lazy7 viewPage
                        model.page
                        st
                        model.ui
                        (ifThen (model.page == "gestures" || model.page == "calibrate") model.now 0)
                        (ifThen (model.page == "gestures") model.flashG Dict.empty)
                        (ifThen (model.page == "settings" && model.ui.tab == "detection") model.history [])
                        (ifThen (List.member model.page [ "gestures", "settings" ]) model.stroke Nothing)
                    ]
                ]


ifThen : Bool -> a -> a -> a
ifThen c a b =
    if c then
        a

    else
        b


sidebar : Model -> State -> Html Msg
sidebar model st =
    nav [ class "sidebar" ]
        [ div [ class "brand" ] [ span [ class "logo" ] [ text "◎" ], text "CoinChord" ]
        , div [ class "nav" ]
            (List.map
                (\( p, icon, label ) ->
                    button [ classList [ ( "nav-item", True ), ( "active", model.page == p ) ], onClick (Go p) ]
                        [ span [ class "icon" ] [ text icon ], text label ]
                )
                pages
            )
        , div [ class "spacer" ] []
        , div [ classList [ ( "output", True ), ( "on", st.output ) ], onClick (Do "output" [ num "value" (ifThen st.output 0 1) ]), title "When on, chords type, click and switch windows on your computer." ]
            [ div [ class "switch" ] [ div [ class "knob" ] [] ]
            , div []
                [ div [ class "output-title" ] [ text (ifThen st.output "Output on" "Output off") ]
                , div [ class "hint" ] [ text (ifThen st.output "Chords act on your computer" "Chords stay in this page") ]
                ]
            ]
        , div [ class "conn" ]
            [ span [ classList [ ( "dot", True ), ( "ok", model.connected ) ] ] []
            , text (ifThen model.connected "Connected" "Reconnecting…")
            ]
        ]


topbar : State -> Html Msg
topbar st =
    header [ class "topbar" ]
        [ div [ class "journey" ]
            (List.map
                (\s -> button [ classList [ ( "stage", True ), ( "done", s.done ) ], onClick (Go s.page) ] [ text (ifThen s.done "✓ " "") , text s.name ])
                st.journey
            )
        , div [ class "next" ]
            [ span [ class "next-label" ] [ text "Next" ]
            , span [ class "next-say" ] [ text st.next.say ]
            , ifThen (st.next.button /= "") (button [ class "btn primary", onClick (DoRaw st.next.cmds) ] [ text (st.next.button ++ " →") ]) (text "")
            ]
        , a [ class "sponsor", href "https://github.com/sponsors/vic", A.target "_blank", A.rel "noopener" ] [ text "♥ Sponsor <3" ]
        ]


notice : State -> Html Msg
notice st =
    if st.notice == "" then
        text ""

    else
        div [ class "notice" ] [ text st.notice, button [ class "link", onClick (Do "notice.clear" []) ] [ text "Dismiss" ] ]



-- LIVE STRIP


liveStrip : Model -> State -> Html Msg
liveStrip model st =
    div [ class "live" ]
        [ div [ class "coins" ] (List.map (coin model) st.kit.voices)
        , micStatus st
        , meter model
        , heard st
        ]


coin : Model -> Voice -> Html Msg
coin model v =
    let
        lit =
            Dict.get v.code model.flashS |> Maybe.map (\t -> model.now - t < flashMs) |> Maybe.withDefault False
    in
    div [ classList [ ( "coin", True ), ( "lit", lit ) ], cvar (Draw.handColor v.hand), title v.name ]
        [ div [ class "disc" ] [ text v.code ], div [ class "coin-name" ] [ text v.name ] ]


micStatus : State -> Html Msg
micStatus st =
    let
        fix label i =
            button [ class "btn small", onClick (Do "mic" [ int "i" i ]) ] [ text label ]

        ( cls, line, fixes ) =
            case st.mic.state of
                "live" ->
                    ( "ok", "Listening", [] )

                "silent" ->
                    ( "bad", "Your mic sends silence — probably muted", [ fix "Unmute" 0 ] )

                "loud" ->
                    ( "warn", "Your mic is too loud", [ fix "Volume −" 1 ] )

                "off" ->
                    ( "bad", "No microphone: " ++ st.mic.error, [] )

                _ ->
                    ( "warn", "Starting the mic…", [] )
    in
    div [ class ("mic " ++ cls) ]
        [ div [ class "mic-line" ] [ span [ class "pulse" ] [], text line ]
        , div [ class "row" ] fixes
        ]


meter : Model -> Html Msg
meter model =
    let
        f =
            model.frame

        pct x =
            String.fromFloat (x * 100) ++ "%"
    in
    div [ class "meter-box", title "How loud the room is right now. A sound has to cross the red line to count." ]
        [ div [ class "meter" ]
            [ div [ classList [ ( "fill", True ), ( "over", f.level > f.trigger ) ], style "width" (pct f.level) ] []
            , div [ class "mark noise", style "left" (pct f.noise) ] []
            , div [ class "mark trigger", style "left" (pct f.trigger) ] []
            ]
        , div [ class "legend" ] [ span [ class "sw noise" ] [], text "room noise", span [ class "sw trigger" ] [], text "a sound must cross this" ]
        , Draw.spectrum f.bands
        ]


heard : State -> Html Msg
heard st =
    div [ class "heard" ]
        (case st.last of
            Nothing ->
                [ div [ class "hint" ] [ text "Tap one of your objects on the desk." ] ]

            Just g ->
                [ div [ class "hint" ] [ text "I heard" ]
                , gchip g
                , case st.chord of
                    Just c ->
                        div [ class "typed" ] [ chordChips c, ifThen (st.typed /= "") (span [] [ text (" → " ++ st.typed) ]) (text "") ]

                    Nothing ->
                        text ""
                ]
        )


gchip : G -> Html msg
gchip g =
    span [ class "gchip", cvar (Draw.handColor g.hand) ] [ text (g.src ++ " " ++ g.glyph ++ " " ++ g.shape) ]


chordChips : Chord -> Html msg
chordChips c =
    span [ class "chord" ] (List.intersperse (span [ class "plus" ] [ text "+" ]) (List.map gchip c.steps))



-- PAGES


viewPage : String -> State -> Ui -> Float -> Dict String Float -> List Float -> Maybe Stroke -> Html Msg
viewPage page st ui now flashG history lastStroke =
    div [ class "page" ]
        [ case page of
            "calibrate" ->
                calibratePage st

            "gestures" ->
                gesturesPage st ui now flashG lastStroke

            "tutor" ->
                tutorPage st

            "layers" ->
                layersPage st

            "settings" ->
                settingsPage st ui history lastStroke

            _ ->
                kitsPage st ui
        ]


card : List (Html msg) -> Html msg
card =
    div [ class "card" ]


stars : Int -> Html msg
stars n =
    span [ class "stars" ] [ text (String.repeat n "★"), span [ class "dim" ] [ text (String.repeat (5 - n) "★") ] ]



-- KITS


kitsPage : State -> Ui -> Html Msg
kitsPage st ui =
    div []
        [ div [ class "hero" ]
            [ h1 [] [ text "Pick what you'll tap with" ]
            , p [] [ text "Hold a small object in each hand and tap, drag, hook and swirl it on the desk. CoinChord tells your objects apart by their sound, and turns each little motion into a whole word, a shortcut or a click." ]
            ]
        , div [ class "two" ]
            [ yourKit st ui
            , card
                [ h3 [] [ text "Saved setups" ]
                , p [ class "hint" ] [ text "Each setup keeps its objects, calibration, scores and chords. Switch with one click." ]
                , if List.isEmpty st.setups then
                    p [ class "hint" ] [ text "None yet. Calibrate a kit, then save it on the left." ]

                  else
                    div [ class "list" ]
                        (List.indexedMap
                            (\i s ->
                                div [ classList [ ( "setup", True ), ( "current", s.name == st.kit.name ) ] ]
                                    [ button [ class "setup-use", onClick (Do "setup.use" [ int "i" i ]) ]
                                        [ strong [] [ text s.name ], span [ class "hint" ] [ text (String.fromInt s.strikers ++ " objects · " ++ ifThen s.calibrated "calibrated" "not calibrated") ] ]
                                    , button [ class "link", onClick (Do "setup.drop" [ int "i" i ]) ] [ text "Delete" ]
                                    ]
                            )
                            st.setups
                        )
                ]
            ]
        , h2 [] [ text "Or start from a kit idea" ]
        , div [ class "grid kits" ] (List.indexedMap (presetCard st) st.presets)
        ]


yourKit : State -> Ui -> Html Msg
yourKit st ui =
    card
        [ div [ class "row between" ]
            [ h3 [] [ text ("Your kit: " ++ st.kit.name) ]
            , span [ classList [ ( "badge", True ), ( "ok", st.kit.calibrated ) ] ] [ text (ifThen st.kit.calibrated "Calibrated" "Not calibrated") ]
            ]
        , div [ class "list" ]
            (List.indexedMap
                (\i v ->
                    div [ class "voice" ]
                        [ span [ class "disc small", cvar (Draw.handColor v.hand) ] [ text v.code ]
                        , input
                            [ value (Dict.get i ui.renames |> Maybe.withDefault v.name)
                            , onInput (\s -> SetUi (\u -> { u | renames = Dict.insert i s u.renames }))
                            , onBlur (Do "kit.rename" [ int "i" i, str "name" (Dict.get i ui.renames |> Maybe.withDefault v.name) ])
                            ]
                            []
                        , button [ class "link", onClick (Do "kit.remove" [ int "i" i ]), title "Remove this object" ] [ text "Remove" ]
                        ]
                )
                st.kit.voices
            )
        , div [ class "row" ]
            [ button [ class "btn", onClick (Do "kit.add" [ int "i" 0 ]) ] [ text "+ Left object" ]
            , button [ class "btn", onClick (Do "kit.add" [ int "i" 1 ]) ] [ text "+ Right object" ]
            ]
        , div [ class "row" ]
            [ input [ placeholder "Name this setup, e.g. coins on the plate", value ui.setupName, onInput (\s -> SetUi (\u -> { u | setupName = s })) ] []
            , button [ class "btn primary", onClick (Do "setup.save" [ str "name" ui.setupName ]) ] [ text "Save setup" ]
            ]
        ]


presetCard : State -> Int -> Preset -> Html Msg
presetCard st i kit =
    button [ classList [ ( "card kit", True ), ( "current", kit.name == st.kit.name ) ], onClick (Do "kit.pick" [ int "i" i ]) ]
        [ h3 [] [ text kit.name ]
        , p [ class "objects" ] [ text (String.join " · " kit.objects) ]
        , p [ class "hint" ] [ text ("On: " ++ kit.surface) ]
        , div [ class "ratings" ]
            [ span [] [ text "Contrast" ], stars kit.contrast, span [] [ text "Shapes" ], stars kit.glide, span [] [ text "Speed" ], stars kit.speed ]
        , p [ class "feel" ] [ text kit.feel ]
        , div [ class "count" ] [ text (String.fromInt kit.chords ++ " chords") ]
        ]



-- CALIBRATE


calSteps : List ( String, String, Int )
calSteps =
    [ ( "quiet", "Quiet room", 1 ), ( "voice", "Your objects", 2 ), ( "drag", "A drag", 3 ), ( "test", "Every gesture", 4 ), ( "report", "Report", 5 ) ]


calibratePage : State -> Html Msg
calibratePage st =
    let
        c =
            st.cal

        order s =
            calSteps |> List.filter (\( n, _, _ ) -> n == s) |> List.head |> Maybe.map (\( _, _, i ) -> i) |> Maybe.withDefault 0
    in
    div []
        [ div [ class "stepper" ]
            (List.map
                (\( n, label, i ) ->
                    button [ classList [ ( "step", True ), ( "active", n == c.step ), ( "done", order c.step > i ) ], onClick (Do "cal.go" [ int "i" i ]) ]
                        [ span [ class "num" ] [ text (ifThen (order c.step > i) "✓" (String.fromInt i)) ], text label ]
                )
                calSteps
            )
        , ifThen (c.msg /= "") (div [ classList [ ( "banner", True ), ( "good", c.good ), ( "bad", not c.good ) ] ] [ text c.msg ]) (text "")
        , case c.step of
            "quiet" ->
                card
                    [ h1 [] [ text "Shh… stay still for two seconds" ]
                    , p [] [ text "CoinChord listens to your room — fans, street, hum — and sets the line a tap must cross." ]
                    , div [ class "progress" ] [ div [ style "width" (String.fromFloat (c.quiet * 100) ++ "%") ] [] ]
                    ]

            "voice" ->
                voiceStep st

            "drag" ->
                let
                    v =
                        List.head st.kit.voices |> Maybe.withDefault (Voice "L" "object" "L")
                in
                card
                    [ div [ class "big-step" ]
                        [ Draw.shape "LINE" v.hand
                        , div []
                            [ h1 [] [ text ("Drag your " ++ v.name ++ " 3 times") ]
                            , p [] [ text "A straight drag across the desk, about an inch. This teaches the difference between a tap and friction." ]
                            , dots c.drags c.dragsNeeded v.hand
                            ]
                        ]
                    ]

            "test" ->
                testStep st

            "report" ->
                report st

            _ ->
                card
                    [ h1 [] [ text "Teach CoinChord your objects" ]
                    , p [] [ text ("Kit: " ++ st.kit.name ++ ". It takes about two minutes and there's no wrong answer.") ]
                    , ol []
                        [ li [] [ text "Stay quiet for two seconds." ]
                        , li [] [ text "Tap each object five times." ]
                        , li [] [ text "Drag one object three times." ]
                        , li [] [ text "Try every gesture and see which ones the mic hears well." ]
                        ]
                    , p [ class "hint" ] [ text "Rest your palms on the desk and hold each object near its edge." ]
                    , div [ class "row" ]
                        [ button [ class "btn primary big", onClick (Do "cal.go" [ int "i" 1 ]) ] [ text "Start →" ]
                        , button [ class "btn", onClick (Do "cal.go" [ int "i" 4 ]) ] [ text "Skip to the gesture test" ]
                        ]
                    ]
        ]


dots : Int -> Int -> String -> Html msg
dots n of_ hand =
    div [ class "dots" ] (List.map (\i -> span [ classList [ ( "dot-big", True ), ( "on", i < n ) ], cvar (Draw.handColor hand) ] []) (List.range 0 (of_ - 1)))


voiceStep : State -> Html Msg
voiceStep st =
    let
        c =
            st.cal

        v =
            st.kit.voices |> List.drop c.voice |> List.head |> Maybe.withDefault (Voice "?" "object" "L")
    in
    div [ class "two" ]
        [ card
            [ div [ class "big-step" ]
                [ Draw.shape "TAP" v.hand
                , div []
                    [ p [ class "hint" ] [ text ("Object " ++ String.fromInt (c.voice + 1) ++ " of " ++ String.fromInt (List.length st.kit.voices)) ]
                    , h1 [] [ text ("Tap your " ++ v.name ++ " 5 times") ]
                    , p [] [ text (ifThen (v.hand == "L") "Left hand." "Right hand." ++ " A relaxed, natural tap. Lift after each one.") ]
                    , dots (c.taps |> List.drop c.voice |> List.head |> Maybe.withDefault 0) c.tapsNeeded v.hand
                    ]
                ]
            ]
        , alikeCard st
        ]


alikeCard : State -> Html Msg
alikeCard st =
    card
        [ h3 [] [ text "How different your objects sound" ]
        , p [ class "hint" ] [ text "The more different, the fewer mix-ups. Above 90% alike, try other objects." ]
        , div [ class "list" ]
            (List.map
                (\x ->
                    div [ class "alike" ]
                        [ span [] [ text (x.a ++ " vs " ++ x.b) ]
                        , div [ class "bar" ] [ div [ classList [ ( "bad", x.sim > 0.9 ), ( "warn", x.sim > 0.75 && x.sim <= 0.9 ) ], style "width" (String.fromFloat (x.sim * 100) ++ "%") ] [] ]
                        , span [ class "num" ] [ text (String.fromInt (round (x.sim * 100)) ++ "% alike") ]
                        ]
                )
                st.cal.alike
            )
        ]


testStep : State -> Html Msg
testStep st =
    let
        c =
            st.cal

        target =
            st.gestures |> List.drop c.target |> List.head
    in
    div []
        [ case target of
            Just t ->
                card
                    [ div [ class "big-step" ]
                        [ Draw.shape t.g.shape t.g.hand
                        , div []
                            [ p [ class "hint" ] [ text ("Gesture " ++ String.fromInt (c.target + 1) ++ " of " ++ String.fromInt (List.length st.gestures)) ]
                            , h1 [] [ gchip t.g ]
                            , p [] [ text t.g.how ]
                            , div [ class "tries" ]
                                (List.map (\ok -> span [ classList [ ( "try", True ), ( "ok", ok ), ( "no", not ok ) ] ] [ text (ifThen ok "✓" "✗") ]) c.attempts
                                    ++ List.repeat (c.triesEach - List.length c.attempts) (span [ class "try" ] [ text "·" ])
                                )
                            , div [ class "row" ]
                                [ button [ class "btn", onClick (Do "cal.skip" []) ] [ text "Skip" ]
                                , button [ class "btn", onClick (Do "cal.go" [ int "i" 5 ]) ] [ text "See the report" ]
                                ]
                            , p [ class "hint" ] [ text "This measures the microphone, not you." ]
                            ]
                        ]
                    ]

            Nothing ->
                text ""
        , h3 [] [ text "All gestures — click one to test it" ]
        , div [ class "grid scores" ]
            (List.indexedMap
                (\i g ->
                    button [ classList [ ( "score", True ), ( "current", i == c.target ) ], onClick (Do "cal.target" [ int "i" i ]) ]
                        [ gchip g.g, scoreText g ]
                )
                st.gestures
            )
        ]


scoreText : GStat -> Html msg
scoreText g =
    if g.tries == 0 then
        span [ class "dim" ] [ text "—" ]

    else
        let
            r =
                toFloat g.hits / toFloat g.tries
        in
        span [ classList [ ( "good", r >= 0.66 ), ( "warn", r < 0.66 && r > 0.3 ), ( "bad", r <= 0.3 ) ] ] [ text (String.fromInt (round (r * 100)) ++ "%") ]


report : State -> Html Msg
report st =
    let
        c =
            st.cal

        weak =
            st.gestures |> List.filter (\g -> g.tries > 0 && toFloat g.hits / toFloat g.tries < 0.66) |> List.take 6
    in
    div []
        [ div [ class "stats" ]
            [ stat (String.fromInt c.reliable) "reliable gestures"
            , stat (String.fromInt c.tested) "tested"
            , stat (String.fromInt c.chords) "chords at your fingertips"
            ]
        , p [ class "lead" ] [ text (ifThen (c.reliable >= 6) "Great start — every reliable gesture multiplies your chords." "Every new kit feels odd at first. A few minutes of practice usually doubles this.") ]
        , div [ class "two" ]
            [ card
                [ h3 [] [ text "Worth practicing" ]
                , if List.isEmpty weak then
                    p [ class "hint" ] [ text "Nothing weak so far." ]

                  else
                    div [ class "list" ] (List.map (\g -> div [ class "weak" ] [ gchip g.g, scoreText g, span [ class "hint" ] [ text g.g.how ] ]) weak)
                ]
            , alikeCard st
            ]
        , div [ class "row" ]
            [ button [ class "btn", onClick (Do "cal.retest" []) ] [ text "Retest the weak ones" ]
            , button [ class "btn", onClick (Do "cal.go" [ int "i" 4 ]) ] [ text "Test more" ]
            , button [ class "btn primary", onClick (Go "tutor") ] [ text "Start the Tutor →" ]
            ]
        ]


stat : String -> String -> Html msg
stat n label =
    div [ class "stat" ] [ div [ class "stat-n" ] [ text n ], div [ class "stat-l" ] [ text label ] ]



-- GESTURES


gesturesPage : State -> Ui -> Float -> Dict String Float -> Maybe Stroke -> Html Msg
gesturesPage st ui now flashG lastStroke =
    let
        found =
            List.length (List.filter (\g -> g.heard > 0) st.gestures)

        lit g =
            Dict.get g.g.name flashG |> Maybe.map (\t -> now - t < flashMs) |> Maybe.withDefault False
    in
    div []
        [ div [ class "hero small" ]
            [ h1 [] [ text "Every gesture your kit can make" ]
            , p [] [ text ("You've found " ++ String.fromInt found ++ " of " ++ String.fromInt (List.length st.gestures) ++ ". Each one you can do is another word at your fingertips.") ]
            ]
        , div [ class "split" ]
            [ div [ class "grid gestures" ]
                (List.map
                    (\g ->
                        button [ classList [ ( "card gesture", True ), ( "lit", lit g ), ( "picked", ui.picked == Just g.g.name ) ], cvar (Draw.handColor g.g.hand), onClick (SetUi (\u -> { u | picked = Just g.g.name })) ]
                            [ Draw.shape g.g.shape g.g.hand
                            , gchip g.g
                            , div [ class "hint" ] [ text (ifThen (g.heard > 0) ("heard " ++ String.fromInt g.heard ++ "×") "not yet") ]
                            ]
                    )
                    st.gestures
                )
            , div [ class "side" ]
                [ case List.filter (\g -> ui.picked == Just g.g.name) st.gestures of
                    g :: _ ->
                        card [ gchip g.g, p [] [ text g.g.how ], p [ class "hint" ] [ text (ifThen (String.contains "&" g.g.src) "Both hands at the same time." "One hand.") ] ]

                    [] ->
                        card [ p [ class "hint" ] [ text "Click a card to see how to do it." ] ]
                , whyCard st lastStroke
                ]
            ]
        ]


whyCard : State -> Maybe Stroke -> Html Msg
whyCard st lastStroke =
    case lastStroke of
        Nothing ->
            card [ h3 [] [ text "How I measured it" ], p [ class "hint" ] [ text "Do any gesture to see what the microphone heard." ] ]

        Just s ->
            card
                [ h3 [] [ text "How I measured it" ]
                , gchip s.gesture
                , gaugeRow "Length" (s.metrics.durMs / 1000) 1.5 (knobValue "Tap max ms" st / 1000) (String.fromInt (round s.metrics.durMs) ++ " ms") "taps are shorter than the line"
                , gaugeRow "Loudness" s.metrics.peak (knobValue "Tap loudness" st * knobValue "Knock ratio" st * 1.5) (knobValue "Tap loudness" st * knobValue "Knock ratio" st) (String.fromFloat (toFixed 2 s.metrics.peak)) "knocks are louder than the line"
                , gaugeRow "Friction" s.metrics.flat 1 (knobValue "Friction flatness" st) (String.fromFloat (toFixed 2 s.metrics.flat)) "drags are past the line"
                , gaugeRow "Bumps" (toFloat s.metrics.bumps) 8 (knobValue "Circle bumps" st) (String.fromInt s.metrics.bumps) "circles have at least this many"
                , gaugeRow "Bend" s.metrics.bendHz 3000 (knobValue "Curve drift Hz" st) (String.fromInt (round s.metrics.bendHz) ++ " Hz") "curves bend past the line"
                , h3 [] [ text "Which object?" ]
                , div [ class "list" ]
                    (List.indexedMap
                        (\i v ->
                            let
                                sim =
                                    s.sims |> List.drop i |> List.head |> Maybe.withDefault 0
                            in
                            div [ class "alike" ]
                                [ span [] [ text v.code ]
                                , div [ class "bar" ] [ div [ style "width" (String.fromFloat (sim * 100) ++ "%"), style "background" (Draw.handColor v.hand) ] [] ]
                                , span [ class "num" ] [ text (ifThen (i == s.best) "◀ this one" "") ]
                                ]
                        )
                        st.kit.voices
                    )
                ]


toFixed : Int -> Float -> Float
toFixed n x =
    toFloat (round (x * 10 ^ toFloat n)) / 10 ^ toFloat n


gaugeRow : String -> Float -> Float -> Float -> String -> String -> Html msg
gaugeRow label v top line shown help =
    let
        pct x =
            String.fromFloat (clamp 0 100 (x / max top 0.0001 * 100)) ++ "%"
    in
    div [ class "gauge", title help ]
        [ span [ class "gauge-label" ] [ text label ]
        , div [ class "bar" ] [ div [ style "width" (pct v) ] [], div [ class "line", style "left" (pct line) ] [] ]
        , span [ class "num" ] [ text shown ]
        ]



-- TUTOR


tutorPage : State -> Html Msg
tutorPage st =
    let
        t =
            st.tutor

        story =
            t.stories |> List.drop t.story |> List.head |> Maybe.withDefault (Story "" [] [])

        best i =
            story.best |> List.drop i |> List.head |> Maybe.withDefault 0

        bestTag i =
            ifThen (best i > 0) (span [ class "tag" ] [ text (String.fromFloat (best i) ++ " wpm") ]) (text "")

        running =
            t.mode /= "pick"
    in
    div [ class "split tutor" ]
        [ div [ class "side left" ]
            [ card
                [ h3 [] [ text "Stories" ]
                , div [ class "list" ] (List.indexedMap (\i s -> button [ classList [ ( "item", True ), ( "current", i == t.story ) ], onClick (Do "tut.story" [ int "i" i ]) ] [ text s.title ]) t.stories)
                , h3 [] [ text "Lessons" ]
                , div [ class "list" ]
                    (List.indexedMap
                        (\i s -> button [ classList [ ( "item", True ), ( "current", running && t.drill < 0 && i == t.lesson ) ], onClick (Do "tut.lesson" [ int "i" i ]) ] [ span [ class "ellipsis" ] [ text s ], bestTag i ])
                        story.sentences
                        ++ [ button [ class "item strong", onClick (Do "tut.lesson" [ int "i" (List.length story.sentences) ]) ] [ text "★ The whole story", bestTag (List.length story.sentences) ] ]
                    )
                , h3 [] [ text "Drive your computer" ]
                , div [ class "list" ] (List.indexedMap (\i d -> button [ classList [ ( "item", True ), ( "current", running && i == t.drill ) ], onClick (Do "tut.drill" [ int "i" i ]), title d.intro ] [ text ("⌘ " ++ d.title) ]) t.drills)
                ]
            ]
        , div []
            [ case t.mode of
                "pick" ->
                    tutorIntro st

                "done" ->
                    tutorDone st

                _ ->
                    tutorRun st
            ]
        ]


tutorIntro : State -> Html Msg
tutorIntro st =
    let
        t =
            st.tutor
    in
    card
        [ h1 [] [ text "Small gestures, whole phrases" ]
        , case t.magic of
            Just m ->
                div [ class "magic" ]
                    [ div [ class "magic-one" ] [ chordChips m.chord, span [ class "arrow" ] [ text "types" ], span [ class "quote" ] [ text ("“" ++ String.replace "·" " " m.show ++ "”") ] ]
                    , p [ class "lead" ] [ strong [] [ text (String.fromInt m.count ++ " small gestures") ], text " type this whole sentence:" ]
                    , blockquote [] [ text m.sentence ]
                    ]

            Nothing ->
                text ""
        , div [ class "row" ]
            [ button [ class "btn primary big", onClick (Do "tut.watch" [ int "i" 0 ]) ] [ text "▶ Watch it" ]
            , button [ class "btn big", onClick (Do "tut.lesson" [ int "i" 0 ]) ] [ text "Try it myself" ]
            ]
        , ifThen (t.msg /= "") (div [ class "banner bad" ] [ text t.msg ]) (text "")
        , p [ class "hint" ] [ text ("Mistakes never type anything here. Say each word out loud as you make its shape. Your own texts go in " ++ st.stories ++ ".") ]
        , ifThen (List.isEmpty t.progress) (text "") (div [] [ h3 [] [ text "Your progress" ], Draw.sparkline t.progress ])
        ]


tutorRun : State -> Html Msg
tutorRun st =
    let
        t =
            st.tutor

        n =
            min (String.length t.typed) (String.length t.target)

        current =
            t.steps |> List.drop t.pos |> List.head

        drill =
            t.drill >= 0
    in
    div []
        [ card
            [ div [ class "row between" ]
                [ h2 [] [ text (ifThen (t.mode == "demo") "Watch" "Your turn") ]
                , span [ class "hint" ] [ text ("chord " ++ String.fromInt (min (t.pos + 1) (List.length t.steps)) ++ " of " ++ String.fromInt (List.length t.steps)) ]
                ]
            , if drill then
                div []
                    [ ifThen st.output (text "") (div [ class "banner" ] [ text "Output is off, so this is practice only. Switch Output on (bottom left) to make these chords move your real windows." ])
                    , div [ class "drill" ]
                        (List.indexedMap
                            (\i s ->
                                div [ classList [ ( "drill-step", True ), ( "done", i < t.pos ), ( "now", i == t.pos ) ] ]
                                    [ span [ class "mark" ] [ text (ifThen (i < t.pos) "✓" (ifThen (i == t.pos) "▶" "")) ], span [ class "say" ] [ text s.say ], chordChips s.chord ]
                            )
                            t.steps
                        )
                    ]

              else
                div [ class "sentence" ] [ span [ class "typed" ] [ text (String.left n t.target) ], span [] [ text (String.dropLeft n t.target) ] ]
            ]
        , case current of
            Just s ->
                card
                    [ div [ class "big-step" ]
                        (List.map (\g -> Draw.shape g.shape g.hand) s.chord.steps
                            ++ [ div []
                                    [ p [ class "hint" ] [ text "Next" ]
                                    , h1 [] [ chordChips s.chord ]
                                    , p [ class "lead" ] [ text (ifThen (s.say /= "") s.say ("types “" ++ String.replace "·" " " s.show ++ "”")) ]
                                    , p [ class "hint" ] [ text (String.join " Then quickly: " (List.map .how s.chord.steps)) ]
                                    ]
                               ]
                        )
                    ]

            Nothing ->
                text ""
        , ifThen (t.msg /= "") (div [ classList [ ( "banner", True ), ( "good", t.good ), ( "bad", not t.good ) ] ] [ text t.msg ]) (text "")
        , ifThen (t.mode == "practice")
            (div [ class "stats" ]
                [ stat (String.fromFloat t.stats.wpm) "words / min"
                , stat (String.fromFloat t.stats.cpm) "chords / min"
                , stat (String.fromInt (round (t.stats.accuracy * 100)) ++ "%") "accuracy"
                , stat (String.fromInt t.stats.wrong) "misses"
                ]
            )
            (text "")
        , div [ class "row" ]
            [ button [ class "btn", onClick (Do "tut.again" []) ] [ text "Restart" ]
            , button [ class "btn", onClick (Do "tut.back" []) ] [ text "All lessons" ]
            ]
        ]


tutorDone : State -> Html Msg
tutorDone st =
    let
        t =
            st.tutor
    in
    card
        [ h1 [] [ text (ifThen (t.last.accuracy >= 0.9) "Excellent run! 🎉" "Done! Every run builds it into your hands.") ]
        , div [ class "stats" ]
            [ stat (String.fromInt (round t.last.wpm)) "words / min"
            , stat (String.fromInt (round t.last.cpm)) "chords / min"
            , stat (String.fromInt (round (t.last.accuracy * 100)) ++ "%") "accuracy"
            , stat (String.fromFloat (toFixed 1 t.last.seconds) ++ "s") "time"
            ]
        , ifThen (List.isEmpty t.misses) (text "") (p [ class "hint" ] [ text ("Practice these: " ++ String.join ", " t.misses) ])
        , Draw.sparkline t.progress
        , div [ class "row" ]
            [ button [ class "btn", onClick (Do "tut.again" []) ] [ text "Again" ]
            , button [ class "btn primary", onClick (Do "tut.next" []) ] [ text "Next →" ]
            , button [ class "btn", onClick (Do "tut.back" []) ] [ text "All lessons" ]
            ]
        ]



-- LAYERS


layerTabs : State -> (String -> Msg) -> Html Msg
layerTabs st go =
    div [ class "tabs" ]
        (List.map
            (\l ->
                button [ classList [ ( "tab", True ), ( "active", st.view == l ) ], onClick (go l) ]
                    [ text l, ifThen (st.sandbox.active == l) (span [ class "live-dot" ] []) (text "") ]
            )
            (st.layers ++ [ "global" ])
        )


layersPage : State -> Html Msg
layersPage st =
    div []
        [ div [ class "hero small" ]
            [ h1 [] [ text "What every chord does" ]
            , p [] [ text "A layer is a set of chords for one job. Click a row to try it, or just perform chords: they type in the sandbox (and on your computer when Output is on)." ]
            ]
        , layerTabs st (\l -> Do "layer.view" [ str "name" l ])
        , div [ class "split" ]
            [ card [ rowsTable st False ]
            , div [ class "side" ]
                [ card
                    [ div [ class "row between" ] [ h3 [] [ text "Sandbox" ], span [ class "badge ok" ] [ text ("typing in " ++ st.sandbox.active) ] ]
                    , div [ class "sandbox" ] [ text st.sandbox.text, span [ class "caret" ] [] ]
                    , ifThen (List.isEmpty st.sandbox.host) (text "") (p [ class "hint" ] [ text ("Computer actions: " ++ String.join "  " st.sandbox.host) ])
                    , div [ class "row" ]
                        [ ifThen (st.view /= st.sandbox.active && st.view /= "global") (button [ class "btn primary", onClick (Do "layer.use" [ str "name" st.view ]) ] [ text ("Type with " ++ st.view) ]) (text "")
                        , button [ class "btn", onClick (Do "sandbox.clear" []) ] [ text "Clear" ]
                        ]
                    ]
                , card
                    [ h3 [] [ text "Switch layers from anywhere" ]
                    , div [ class "list" ] (List.map (\r -> div [ class "rowline" ] [ chordChips r.chord, span [] [ text (String.replace "{layer:" "→ " (String.replace "}" "" r.src)) ] ]) st.switches)
                    ]
                ]
            ]
        ]


rowsTable : State -> Bool -> Html Msg
rowsTable st editable =
    if List.isEmpty st.rows then
        p [ class "hint" ] [ text "No chords in this layer yet." ]

    else
        table [ class "rows" ]
            [ tbody []
                (List.map
                    (\r ->
                        tr [ classList [ ( "unreachable", not r.reachable ) ], onClick (ifThen editable (SetUi (\u -> { u | editing = Just r.chord.name, bindText = r.src })) (Do "layer.play" [ str "chord" r.chord.name ])) ]
                            [ td [] [ chordChips r.chord ]
                            , td [ class "show" ] [ text r.show ]
                            , ifThen editable (td [] [ button [ class "link", Html.Events.stopPropagationOn "click" (D.succeed ( Do "bind.delete" [ str "chord" r.chord.name ], True )) ] [ text "Delete" ] ]) (text "")
                            ]
                    )
                    st.rows
                )
            ]



-- SETTINGS


settingsPage : State -> Ui -> List Float -> Maybe Stroke -> Html Msg
settingsPage st ui history lastStroke =
    div []
        [ div [ class "tabs" ]
            (List.map
                (\( k, label ) -> button [ classList [ ( "tab", True ), ( "active", ui.tab == k ) ], onClick (SetUi (\u -> { u | tab = k })) ] [ text label ])
                [ ( "detection", "Detection" ), ( "chords", "Your chords" ), ( "texts", "Practice texts" ) ]
            )
        , case ui.tab of
            "chords" ->
                chordsTab st ui

            "texts" ->
                textsTab st

            _ ->
                detectionTab st ui history lastStroke
        ]


graphHeight : Float
graphHeight =
    200


detectionTab : State -> Ui -> List Float -> Maybe Stroke -> Html Msg
detectionTab st ui history lastStroke =
    let
        trig =
            ui.drag |> Maybe.withDefault (levelOf (knobValue "Trigger level" st))

        n =
            toFloat (max 1 (historyLength - 1))

        y l =
            String.fromFloat (100 - l * 100)

        pts =
            history |> List.reverse |> List.indexedMap (\i l -> String.fromFloat (toFloat (historyLength - List.length history + i) * 100 / n) ++ "," ++ y l)

        noise =
            List.sort history |> List.drop (List.length history // 5) |> List.head |> Maybe.withDefault 0

        yOf =
            D.map (\oy -> DragAt (1 - oy / graphHeight)) (D.field "offsetY" D.float)
    in
    div []
        [ card
            [ div [ class "row between" ]
                [ div []
                    [ h3 [] [ text "When does a sound count?" ]
                    , p [ class "hint" ] [ text "Drag the red line. Sounds that rise above it become strokes. Keep it above the yellow room noise and below your taps." ]
                    ]
                , button [ class "btn", onClick (Do "trigger.auto" []), title "Set the line a little above the noise of the last few seconds. Stay quiet first." ] [ text "Set from room noise" ]
                ]
            , div
                [ class "graph"
                , style "height" (String.fromFloat graphHeight ++ "px")
                , on "mousedown" yOf
                , on "mousemove" (D.field "buttons" D.int |> D.andThen (\b -> ifThen (b == 1) yOf (D.fail "not dragging")))
                , on "mouseup" (D.succeed DragEnd)
                , on "mouseleave" (D.succeed DragEnd)
                ]
                [ Svg.svg [ SA.viewBox "0 0 100 100", SA.preserveAspectRatio "none" ]
                    [ Svg.rect [ SA.x "0", SA.y (y trig), SA.width "100", SA.height (String.fromFloat (trig * 100)), SA.class "below" ] []
                    , Svg.polyline [ SA.points (String.join " " pts), SA.class "level" ] []
                    , Svg.line [ SA.x1 "0", SA.x2 "100", SA.y1 (y noise), SA.y2 (y noise), SA.class "noise" ] []
                    , Svg.line [ SA.x1 "0", SA.x2 "100", SA.y1 (y trig), SA.y2 (y trig), SA.class "trigger" ] []
                    ]
                , div [ class "graph-label", style "top" (String.fromFloat ((1 - trig) * graphHeight - 22) ++ "px") ] [ text ("trigger " ++ String.fromInt (round (trig * 90 - 90)) ++ " dB  ⇕ drag") ]
                ]
            ]
        , card
            [ div [ class "row between" ]
                [ div [] [ h3 [] [ text "Shape thresholds" ], p [ class "hint" ] [ text "Slide a line to move the border between two shapes. The dot is your last stroke." ] ]
                , button [ class "btn", onClick (Do "knobs.reset" []) ] [ text "Reset (keeps calibration)" ]
                ]
            , div [ class "knobs" ] (List.indexedMap (knobRow ui st lastStroke) st.knobs |> List.filter (always True))
            ]
        ]


{-| The measured value of the last stroke that a knob compares against. -}
metricFor : State -> String -> Stroke -> Maybe Float
metricFor st name s =
    let
        m =
            s.metrics
    in
    case name of
        "Trigger level" ->
            Nothing

        "Tap loudness" ->
            Just m.peak

        "Friction flatness" ->
            Just m.flat

        "Tap max ms" ->
            Just m.durMs

        "Slide ms" ->
            Just m.durMs

        "Knock ratio" ->
            Just (m.peak / max 0.0001 (knobValue "Tap loudness" st))

        "Curve drift Hz" ->
            Just m.bendHz

        "Circle bumps" ->
            Just (toFloat m.bumps)

        "Zigzag bumps" ->
            Just (toFloat m.bumps)

        "Zigzag max period ms" ->
            ifThen (m.bumps >= 2) (Just m.periodMs) Nothing

        _ ->
            Nothing


knobRow : Ui -> State -> Maybe Stroke -> Int -> Knob -> Html Msg
knobRow ui st lastStroke i k =
    let
        v =
            Dict.get i ui.knobs |> Maybe.withDefault k.value

        pos x =
            String.fromFloat (clamp 0 100 ((x - k.min) / (k.max - k.min) * 100)) ++ "%"

        shown x =
            ifThen (k.step >= 1) (String.fromInt (round x)) (String.fromFloat (toFixed 3 x))

        marker =
            lastStroke |> Maybe.andThen (metricFor st k.name)
    in
    div [ class "knob-row" ]
        [ div [ class "knob-head" ] [ strong [] [ text k.name ], span [ class "num" ] [ text (shown v) ] ]
        , div [ class "slider" ]
            [ input
                [ type_ "range"
                , A.min (String.fromFloat k.min)
                , A.max (String.fromFloat k.max)
                , A.step (String.fromFloat k.step)
                , value (String.fromFloat v)
                , onInput (\s -> SetUi (\u -> { u | knobs = Dict.insert i (String.toFloat s |> Maybe.withDefault v) u.knobs }))
                , on "change" (D.at [ "target", "value" ] D.string |> D.map (\s -> Do "knob" [ int "i" i, num "value" (String.toFloat s |> Maybe.withDefault v) ]))
                ]
                []
            , case marker of
                Just x ->
                    div [ class "marker", style "left" (pos x), title ("your last stroke: " ++ shown x) ] []

                Nothing ->
                    text ""
            ]
        , div [ class "hint" ] [ text k.help ]
        ]


chordsTab : State -> Ui -> Html Msg
chordsTab st ui =
    let
        editor chordName =
            card
                [ h3 [] [ text ("What should " ++ chordName ++ " do in " ++ st.view ++ "?") ]
                , input [ class "wide", placeholder "text, {enter}, {ctrl+alt+left}, {do:switch}, {scroll:3}, {layer:kbd}…", value ui.bindText, onInput (\s -> SetUi (\u -> { u | bindText = s })) ] []
                , p [ class "hint" ] [ text ("Keys: {enter} {bs} {tab} {esc} {left} {ctrl+c}. Commands: " ++ String.join " " (List.map (\c -> "{do:" ++ c ++ "}") (List.take 8 st.commands)) ++ " … Mouse: {move:40,0} {scroll:3} {click:left}.") ]
                , div [ class "row" ]
                    [ button [ class "btn primary", onClick (Do "bind.save" [ str "chord" chordName, str "text" ui.bindText ]) ] [ text "Save" ]
                    , button [ class "btn", onClick (SetUi (\u -> { u | editing = Nothing })) ] [ text "Cancel" ]
                    ]
                ]
    in
    div []
        [ layerTabs st (\l -> Do "layer.view" [ str "name" l ])
        , div [ class "split" ]
            [ card [ p [ class "hint" ] [ text "Click a row to change it." ], rowsTable st True ]
            , div [ class "side" ]
                [ case ( st.rec.listening, st.rec.chord, ui.editing ) of
                    ( True, _, _ ) ->
                        card [ div [ class "recording" ] [ span [ class "pulse big" ] [], h2 [] [ text "Perform the chord now…" ] ], button [ class "btn", onClick (Do "bind.cancel" []) ] [ text "Cancel" ] ]

                    ( _, Just c, _ ) ->
                        div [] [ card [ p [ class "hint" ] [ text "You performed" ], chordChips c ], editor c.name ]

                    ( _, _, Just name ) ->
                        editor name

                    _ ->
                        card
                            [ h3 [] [ text "Make a new chord" ]
                            , p [ class "hint" ] [ text ("It goes into the " ++ st.view ++ " layer.") ]
                            , button [ class "btn primary big", onClick (Do "bind.record" []) ] [ text "● Record a chord" ]
                            ]
                , card
                    [ h3 [] [ text "New layer" ]
                    , div [ class "row" ]
                        [ input [ placeholder "name, e.g. email", value ui.layerName, onInput (\s -> SetUi (\u -> { u | layerName = s })) ] []
                        , button [ class "btn", onClick (Do "layer.new" [ str "name" ui.layerName ]) ] [ text "Create" ]
                        ]
                    , button [ class "link danger", onClick (ifThen ui.confirmReset (Do "layers.reset" []) (SetUi (\u -> { u | confirmReset = True }))) ]
                        [ text (ifThen ui.confirmReset "Click again: this replaces all your chords with the defaults" "Reset all layers to the defaults") ]
                    ]
                ]
            ]
        ]


textsTab : State -> Html Msg
textsTab st =
    card
        [ h3 [] [ text "Practice with your own texts" ]
        , p [] [ text ("Put any .txt file in " ++ st.stories ++ ", then click Reload.") ]
        , button [ class "btn", onClick (Do "stories.reload" []) ] [ text "Reload texts" ]
        , h3 [] [ text (String.fromInt (List.length st.missing) ++ " words without their own chord") ]
        , p [ class "hint" ] [ text "The tutor spells these letter by letter. Give each one its own chord to type it in one motion." ]
        , p [ class "words" ] [ text (String.join " " (List.take 80 st.missing)) ]
        , ifThen (List.isEmpty st.missing) (text "") (button [ class "btn primary", onClick (Do "words.add" []) ] [ text "Give every word a chord" ])
        ]



-- MAIN


main : Program String Model Msg
main =
    Browser.element
        { init = init
        , update = update
        , view = view
        , subscriptions = \_ -> Sub.batch [ receive Got, Time.every 100 Tick ]
        }


{-| Sets the CSS custom property --c (Html.Attributes.style cannot set custom properties). -}
cvar : String -> Html.Attribute msg
cvar c =
    A.attribute "style" ("--c:" ++ c)
