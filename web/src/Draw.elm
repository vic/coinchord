module Draw exposing (handColor, shape, sparkline, spectrum)

{-| Small SVG drawings: gesture shapes, the spectrum and sparklines. -}

import Html exposing (Html)
import Svg exposing (..)
import Svg.Attributes exposing (..)


handColor : String -> String
handColor h =
    case h of
        "L" ->
            "var(--left)"

        "R" ->
            "var(--right)"

        _ ->
            "var(--both)"


arrowHead : String -> String -> Svg msg
arrowHead hand c =
    defs [] [ marker [ id ("arrow-" ++ hand), viewBox "0 0 10 10", refX "5", refY "5", markerWidth "5", markerHeight "5", orient "auto-start-reverse" ] [ Svg.path [ d "M0 0 L10 5 L0 10 z", fill c ] [] ] ]


{-| A 64x64 drawing of a shape ("TAP", "LINE", ...) in the hand color. -}
shape : String -> String -> Html msg
shape name hand =
    let
        c =
            handColor hand

        mk =
            "url(#arrow-" ++ hand ++ ")"

        stroke_ p =
            Svg.path [ d p, fill "none", stroke c, strokeWidth "4", strokeLinecap "round", strokeLinejoin "round", markerEnd mk ] []

        dot r =
            circle [ cx "32", cy "40", Svg.Attributes.r (String.fromFloat r), fill c ] []
    in
    svg [ viewBox "0 0 64 64", class "shape" ]
        (arrowHead hand c
            :: (case name of
                    "TAP" ->
                        [ stroke_ "M32 8 L32 26", dot 6 ]

                    "KNOCK" ->
                        [ stroke_ "M24 6 L28 24", stroke_ "M40 6 L36 24", dot 9, circle [ cx "32", cy "40", r "15", fill "none", stroke c, strokeWidth "2", strokeDasharray "3 4" ] [] ]

                    "LINE" ->
                        [ circle [ cx "14", cy "32", r "4", fill c ] [], stroke_ "M14 32 L50 32" ]

                    "SLIDE" ->
                        [ circle [ cx "6", cy "32", r "4", fill c ] [], stroke_ "M6 32 L58 32", Svg.path [ d "M6 40 L58 40", stroke c, strokeWidth "2", strokeDasharray "2 4" ] [] ]

                    "CURV" ->
                        [ circle [ cx "12", cy "18", r "4", fill c ] [], stroke_ "M12 18 L40 18 Q52 18 52 32 L52 50" ]

                    "CIRC" ->
                        [ circle [ cx "32", cy "32", r "3", fill c ] [], stroke_ "M32 12 A20 20 0 1 1 14 24" ]

                    _ ->
                        [ circle [ cx "8", cy "36", r "4", fill c ] [], stroke_ "M8 36 L18 24 L28 44 L38 24 L48 44 L56 32" ]
               )
        )


{-| Spectrum bars from 0..1 levels. -}
spectrum : List Float -> Html msg
spectrum levels =
    let
        n =
            toFloat (Basics.max 1 (List.length levels))

        w =
            100 / n
    in
    svg [ viewBox "0 0 100 30", preserveAspectRatio "none", class "spectrum" ]
        (List.indexedMap
            (\i l ->
                rect [ x (String.fromFloat (toFloat i * w)), y (String.fromFloat (30 - 30 * l)), width (String.fromFloat (w * 0.8)), height (String.fromFloat (30 * l)), rx "0.5" ] []
            )
            levels
        )


{-| A line through values scaled to their max. -}
sparkline : List Float -> Html msg
sparkline vs =
    let
        top =
            Maybe.withDefault 1 (List.maximum (1 :: vs))

        n =
            Basics.max 1 (List.length vs - 1)

        pts =
            List.indexedMap (\i v -> String.fromFloat (toFloat i * 100 / toFloat n) ++ "," ++ String.fromFloat (30 - 28 * v / top)) vs
    in
    svg [ viewBox "0 0 100 30", preserveAspectRatio "none", class "sparkline" ]
        [ polyline [ points (String.join " " pts), fill "none", stroke "var(--accent)", strokeWidth "1.5", Svg.Attributes.style "vector-effect: non-scaling-stroke" ] [] ]
