module Main exposing (..)

import Browser
import Html exposing (Html, button, div, h1, h2, input, li, text, ul)
import Html.Attributes exposing (placeholder, value)
import Html.Events exposing (onClick, onInput)
import Http
import Json.Decode as Decode
import Json.Encode as Encode



-- MODEL


type alias Model =
    { players : List String
    , playerName : String
    , gameId : Int
    , maxPoints : String
    , scores : List ( String, Int )
    , newScore : Int
    , error : Maybe String
    }


initialModel : Model
initialModel =
    { players = []
    , playerName = ""
    , gameId = 0
    , maxPoints = ""
    , scores = []
    , newScore = 0
    , error = Nothing
    }



-- MESSAGES


type Msg
    = SetPlayerName String
    | AddPlayer
    | CreateGame
    | SetMaxPoints String
    | FetchGamePlayers Int
    | AddScore Int
    | ReceivePlayers (Result Http.Error (List String))
    | ReceiveGamePlayers (Result Http.Error (List ( String, Int )))
    | ReceiveGame (Result Http.Error Int)
    | ReceiveError String



-- UPDATE


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        SetPlayerName name ->
            setPlayerName name model

        AddPlayer ->
            addPlayer model

        CreateGame ->
            createGame model

        SetMaxPoints points ->
            setMaxPoints points model

        FetchGamePlayers _ ->
            fetchGamePlayers model

        AddScore score ->
            addScore score model

        ReceivePlayers result ->
            case result of
                Ok newPlayers ->
                    ( { model | players = model.players ++ newPlayers, playerName = "" }, Cmd.none )

                Err _ ->
                    ( { model | error = Just "Failed to add player." }, Cmd.none )

        ReceiveGamePlayers result ->
            case result of
                Ok playerScores ->
                    ( { model | scores = playerScores }, Cmd.none )

                Err _ ->
                    ( { model | error = Just "Failed to fetch game players." }, Cmd.none )

        ReceiveGame result ->
            case result of
                Ok gameId ->
                    ( { model | gameId = gameId }, Cmd.none )

                Err _ ->
                    ( { model | error = Just "Failed to create game." }, Cmd.none )

        ReceiveError message ->
            ( { model | error = Just message }, Cmd.none )


setPlayerName : String -> Model -> ( Model, Cmd Msg )
setPlayerName name model =
    ( { model | playerName = name }, Cmd.none )


addPlayer : Model -> ( Model, Cmd Msg )
addPlayer model =
    let
        body =
            Encode.object [ ( "name", Encode.string model.playerName ) ]
    in
    ( model
    , Http.post
        { url = "http://localhost:8080/players/add"
        , body = Http.jsonBody body
        , expect = Http.expectJson ReceivePlayers playerDecoder
        }
    )


setMaxPoints : String -> Model -> ( Model, Cmd Msg )
setMaxPoints points model =
    ( { model | maxPoints = points }, Cmd.none )


createGame : Model -> ( Model, Cmd Msg )
createGame model =
    case String.toInt model.maxPoints of
        Just points ->
            let
                body =
                    Encode.object [ ( "max_points", Encode.int points ) ]
            in
            ( model
            , Http.post
                { url = "http://localhost:8080/games/new"
                , body = Http.jsonBody body
                , expect = Http.expectJson ReceiveGame gameDecoder
                }
            )

        Nothing ->
            ( { model | error = Just "invalid max points" }, Cmd.none )


fetchGamePlayers : Model -> ( Model, Cmd Msg )
fetchGamePlayers model =
    let
        url =
            "http://localhost:8080/games?game_id=" ++ String.fromInt model.gameId
    in
    ( model
    , Http.get
        { url = url
        , expect = Http.expectJson ReceiveGamePlayers gamePlayersDecoder
        }
    )


addScore : Int -> Model -> ( Model, Cmd Msg )
addScore score model =
    let
        body =
            Encode.object
                [ ( "game_id", Encode.int model.gameId )
                , ( "player_id", Encode.int 1 ) -- Replace with dynamic player_id
                , ( "score", Encode.int score )
                ]
    in
    ( model
    , Http.request
        { method = "PUT"
        , headers = []
        , url = "http://localhost:8080/games/update-score"
        , body = Http.jsonBody body
        , expect = Http.expectString (always (ReceiveError "Failed to update score"))
        , timeout = Nothing
        , tracker = Nothing
        }
    )



-- DECODERS


playerDecoder : Decode.Decoder (List String)
playerDecoder =
    Decode.list (Decode.field "name" Decode.string)


gameDecoder : Decode.Decoder Int
gameDecoder =
    Decode.field "id" Decode.int


gamePlayersDecoder : Decode.Decoder (List ( String, Int ))
gamePlayersDecoder =
    Decode.list
        (Decode.map2 Tuple.pair
            (Decode.field "player_name" Decode.string)
            (Decode.field "score" Decode.int)
        )



-- VIEW


view : Model -> Html Msg
view model =
    div []
        [ h1 [] [ text "Canastra" ]
        , viewAddPlayer model
        , viewCreateGame model

        -- , viewScores model
        ]


viewAddPlayer : Model -> Html Msg
viewAddPlayer model =
    div []
        [ h2 [] [ text "Add Player" ]
        , input [ placeholder "Player Name", value model.playerName, onInput SetPlayerName ] []
        , button [ onClick AddPlayer ] [ text "Add Player" ]
        , ul [] (List.map (\player -> li [] [ text player ]) model.players)
        ]


viewCreateGame : Model -> Html Msg
viewCreateGame model =
    div []
        [ h2 [] [ text "Create Game" ]
        , input [ placeholder "Max Points", value model.maxPoints, onInput SetMaxPoints ] []
        , button [ onClick CreateGame ] [ text "Create Game" ]
        ]


viewScores : Model -> Html Msg
viewScores model =
    div []
        [ h2 [] [ text "Scores" ]
        , ul []
            (List.map
                (\( player, score ) -> li [] [ text (player ++ ": " ++ String.fromInt score) ])
                model.scores
            )
        , input [ placeholder "Add Score", onInput (\score -> AddScore (String.toInt score |> Maybe.withDefault 0)) ] []
        ]



-- MAIN


main : Program () Model Msg
main =
    Browser.element { init = \_ -> ( initialModel, Cmd.none ), update = update, view = view, subscriptions = \_ -> Sub.none }
