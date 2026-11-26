module Api exposing (..)

{-| The messages the engine sends. Field names match internal/app/state.go.
Go encodes empty lists as null, so every list goes through `list`.
-}

import Json.Decode as D exposing (Decoder)
import Json.Decode.Pipeline exposing (optional, required)


list : Decoder a -> Decoder (List a)
list d =
    D.oneOf [ D.list d, D.null [] ]


type alias G =
    { name : String, src : String, shape : String, hand : String, glyph : String, how : String }


gesture : Decoder G
gesture =
    D.succeed G
        |> required "name" D.string
        |> required "src" D.string
        |> required "shape" D.string
        |> required "hand" D.string
        |> required "glyph" D.string
        |> required "how" D.string


type alias Chord =
    { name : String, steps : List G }


chord : Decoder Chord
chord =
    D.succeed Chord
        |> required "name" D.string
        |> required "steps" (list gesture)


type alias Frame =
    { level : Float, peak : Float, noise : Float, trigger : Float, bands : List Float }


frame : Decoder Frame
frame =
    D.succeed Frame
        |> required "level" D.float
        |> required "peak" D.float
        |> required "noise" D.float
        |> required "trigger" D.float
        |> required "bands" (list D.float)


type alias Metrics =
    { durMs : Float, peak : Float, flat : Float, bumps : Int, bendHz : Float, periodMs : Float }


type alias Stroke =
    { gesture : G, sims : List Float, best : Int, metrics : Metrics }


stroke : Decoder Stroke
stroke =
    D.succeed Stroke
        |> required "gesture" gesture
        |> required "sims" (list D.float)
        |> required "best" D.int
        |> required "metrics"
            (D.succeed Metrics
                |> required "durMs" D.float
                |> required "peak" D.float
                |> required "flat" D.float
                |> required "bumps" D.int
                |> required "bendHz" D.float
                |> required "periodMs" D.float
            )


type alias Voice =
    { code : String, name : String, hand : String }


type alias Preset =
    { name : String, surface : String, feel : String, objects : List String, contrast : Int, glide : Int, speed : Int, gestures : Int, chords : Int }


type alias Setup =
    { name : String, calibrated : Bool, strikers : Int }


type alias Knob =
    { name : String, help : String, min : Float, max : Float, step : Float, value : Float }


type alias GStat =
    { g : G, heard : Int, hits : Int, tries : Int }


type alias Mic =
    { state : String, error : String, volume : Float, muted : Bool }


type alias Next =
    { say : String, button : String, cmds : List D.Value }


type alias Stage =
    { name : String, done : Bool, page : String }


type alias Alike =
    { a : String, b : String, sim : Float }


type alias Cal =
    { step : String
    , voice : Int
    , taps : List Int
    , tapsNeeded : Int
    , drags : Int
    , dragsNeeded : Int
    , quiet : Float
    , msg : String
    , good : Bool
    , attempts : List Bool
    , triesEach : Int
    , target : Int
    , alike : List Alike
    , reliable : Int
    , tested : Int
    , chords : Int
    }


type alias Story =
    { title : String, sentences : List String, best : List Float }


type alias Drill =
    { title : String, intro : String }


type alias Step =
    { chord : Chord, show : String, say : String }


type alias Stats =
    { wpm : Float, cpm : Float, accuracy : Float, wrong : Int }


type alias Record =
    { wpm : Float, cpm : Float, accuracy : Float, seconds : Float }


type alias Magic =
    { chord : Chord, show : String, count : Int, sentence : String }


type alias Tutor =
    { mode : String
    , story : Int
    , lesson : Int
    , drill : Int
    , stories : List Story
    , drills : List Drill
    , steps : List Step
    , pos : Int
    , typed : String
    , target : String
    , msg : String
    , good : Bool
    , stats : Stats
    , last : Record
    , progress : List Float
    , misses : List String
    , magic : Maybe Magic
    }


type alias Row =
    { chord : Chord, show : String, src : String, reachable : Bool }


type alias Sandbox =
    { active : String, text : String, host : List String }


type alias Rec =
    { listening : Bool, chord : Maybe Chord }


type alias State =
    { kit : { name : String, calibrated : Bool, voices : List Voice }
    , presets : List Preset
    , setups : List Setup
    , knobs : List Knob
    , gestures : List GStat
    , output : Bool
    , mic : Mic
    , notice : String
    , next : Next
    , journey : List Stage
    , cal : Cal
    , tutor : Tutor
    , layers : List String
    , view : String
    , rows : List Row
    , switches : List Row
    , sandbox : Sandbox
    , rec : Rec
    , last : Maybe G
    , chord : Maybe Chord
    , typed : String
    , stories : String
    , missing : List String
    , commands : List String
    }


state : Decoder State
state =
    D.succeed State
        |> required "kit"
            (D.map3 (\n c v -> { name = n, calibrated = c, voices = v })
                (D.field "name" D.string)
                (D.field "calibrated" D.bool)
                (D.field "voices" (list (D.map3 Voice (D.field "code" D.string) (D.field "name" D.string) (D.field "hand" D.string))))
            )
        |> required "presets"
            (list
                (D.succeed Preset
                    |> required "name" D.string
                    |> required "surface" D.string
                    |> required "feel" D.string
                    |> required "objects" (list D.string)
                    |> required "contrast" D.int
                    |> required "glide" D.int
                    |> required "speed" D.int
                    |> required "gestures" D.int
                    |> required "chords" D.int
                )
            )
        |> required "setups" (list (D.map3 Setup (D.field "name" D.string) (D.field "calibrated" D.bool) (D.field "strikers" D.int)))
        |> required "knobs"
            (list
                (D.succeed Knob
                    |> required "name" D.string
                    |> required "help" D.string
                    |> required "min" D.float
                    |> required "max" D.float
                    |> required "step" D.float
                    |> required "value" D.float
                )
            )
        |> required "gestures" (list (D.map4 GStat (D.field "g" gesture) (D.field "heard" D.int) (D.field "hits" D.int) (D.field "tries" D.int)))
        |> required "output" D.bool
        |> required "mic" (D.map4 Mic (D.field "state" D.string) (D.field "error" D.string) (D.field "volume" D.float) (D.field "muted" D.bool))
        |> required "notice" D.string
        |> required "next" (D.map3 Next (D.field "say" D.string) (D.field "button" D.string) (D.field "cmds" (list D.value)))
        |> required "journey" (list (D.map3 Stage (D.field "name" D.string) (D.field "done" D.bool) (D.field "page" D.string)))
        |> required "cal"
            (D.succeed Cal
                |> required "step" D.string
                |> required "voice" D.int
                |> required "taps" (list D.int)
                |> required "tapsNeeded" D.int
                |> required "drags" D.int
                |> required "dragsNeeded" D.int
                |> required "quiet" D.float
                |> required "msg" D.string
                |> required "good" D.bool
                |> required "attempts" (list D.bool)
                |> required "triesEach" D.int
                |> required "target" D.int
                |> required "alike" (list (D.map3 Alike (D.field "a" D.string) (D.field "b" D.string) (D.field "sim" D.float)))
                |> required "reliable" D.int
                |> required "tested" D.int
                |> required "chords" D.int
            )
        |> required "tutor" tutor
        |> required "layers" (list D.string)
        |> required "view" D.string
        |> required "rows" (list row)
        |> required "switches" (list row)
        |> required "sandbox" (D.map3 Sandbox (D.field "active" D.string) (D.field "text" D.string) (D.field "host" (list D.string)))
        |> required "rec" (D.map2 Rec (D.field "listening" D.bool) (D.maybe (D.field "chord" chord)))
        |> optional "last" (D.nullable gesture) Nothing
        |> optional "chord" (D.nullable chord) Nothing
        |> required "typed" D.string
        |> required "stories" D.string
        |> required "missing" (list D.string)
        |> required "commands" (list D.string)


row : Decoder Row
row =
    D.map4 Row (D.field "chord" chord) (D.field "show" D.string) (D.field "src" D.string) (D.field "reachable" D.bool)


tutor : Decoder Tutor
tutor =
    D.succeed Tutor
        |> required "mode" D.string
        |> required "story" D.int
        |> required "lesson" D.int
        |> required "drill" D.int
        |> required "stories" (list (D.map3 Story (D.field "title" D.string) (D.field "sentences" (list D.string)) (D.field "best" (list D.float))))
        |> required "drills" (list (D.map2 Drill (D.field "title" D.string) (D.field "intro" D.string)))
        |> required "steps" (list (D.map3 Step (D.field "chord" chord) (D.field "show" D.string) (D.field "say" D.string)))
        |> required "pos" D.int
        |> required "typed" D.string
        |> required "target" D.string
        |> required "msg" D.string
        |> required "good" D.bool
        |> required "stats" (D.map4 Stats (D.field "wpm" D.float) (D.field "cpm" D.float) (D.field "accuracy" D.float) (D.field "wrong" D.int))
        |> required "last" (D.map4 Record (D.field "wpm" D.float) (D.field "cpm" D.float) (D.field "accuracy" D.float) (D.field "seconds" D.float))
        |> required "progress" (list D.float)
        |> required "misses" (list D.string)
        |> optional "magic"
            (D.nullable (D.map4 Magic (D.field "chord" chord) (D.field "show" D.string) (D.field "count" D.int) (D.field "sentence" D.string)))
            Nothing
