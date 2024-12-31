package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var PORT = os.Getenv("PORT")

var db *sql.DB

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./canastra.db")
	if err != nil {
		log.Fatal(err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS players (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS games (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		maximum_score INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS game_players (
		game_id INTEGER NOT NULL,
		player_id INTEGER NOT NULL,
		score INTEGER DEFAULT 0,
		PRIMARY KEY (player_id, game_id)
		FOREIGN KEY(player_id) REFERENCES players(id),
		FOREIGN KEY(game_id) REFERENCES games(id)
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Fatal(err)
	}
}

type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Game struct {
	ID           int    `json:"id"`
	MaximumScore string `json:"maximum_score"`
}

type GamePlayer struct {
	PlayerId int `json:"player_id"`
	GameId   int `json:"game_id"`
	Score    int `json:"score"`
}

type Response struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func main() {
	initDB()
	defer db.Close()

	server := &http.Server{
		Addr: fmt.Sprintf(":%s", PORT),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Response{"Player created", true})
	})

	http.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		var requestData struct {
			MaximumScore int `json:"maximum_score"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"invalid_json", false})
			return
		}

		result, err := db.Exec("INSERT INTO games (maximum_score) VALUES (?)", requestData.MaximumScore)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		gameCreatedResponse := struct {
			GameId  int64  `json:"id"`
			Message string `json:"message"`
			Success bool   `json:"success"`
		}{
			id, "game_created", true,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gameCreatedResponse)
	})

	http.HandleFunc("/games", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		id := r.URL.Query().Get("game_id")

		rows, err := db.Query("SELECT game_id, player_id, score FROM game_players WHERE game_id = ?", id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}
		defer rows.Close()

		gamePlayers := make([]GamePlayer, 0)

		for rows.Next() {
			gamePlayer := new(GamePlayer)
			err := rows.Scan(&gamePlayer.GameId, &gamePlayer.PlayerId, &gamePlayer.Score)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(Response{"internal_server_error", false})
				return
			}
			gamePlayers = append(gamePlayers, *gamePlayer)
		}

		if err = rows.Err(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		var gameResponse = struct {
			Data []GamePlayer `json:"data"`
		}{gamePlayers}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gameResponse)
	})

	http.HandleFunc("/games/update-score", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		var requestData struct {
			GameId   int `json:"game_id"`
			PlayerId int `json:"player_id"`
			Score    int `json:"score"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"invalid_json", false})
			return
		}

		_, err = db.Exec("UPDATE game_players SET score = score + ? WHERE game_id = ? AND player_id = ?", requestData.Score, requestData.GameId, requestData.PlayerId)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		row := db.QueryRow("SELECT score FROM game_players WHERE game_id = ? AND player_id = ?", requestData.GameId, requestData.PlayerId)
		gamePlayer := new(GamePlayer)

		err = row.Scan(&gamePlayer.Score)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		gameUpdatedResponse := struct {
			NewScore int    `json:"new_score"`
			Message  string `json:"message"`
			Success  bool   `json:"success"`
		}{
			gamePlayer.Score, "game_updated", true,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gameUpdatedResponse)
	})

	http.HandleFunc("/games/players/add", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		var requestData struct {
			PlayerId int `json:"player_id"`
			GameId   int `json:"game_id"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"invalid_json", false})
			return
		}

		result, err := db.Exec("INSERT INTO game_players (player_id, game_id) VALUES (?, ?)", requestData.PlayerId, requestData.GameId)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		gamePlayersCreatedResponse := struct {
			ID      int64  `json:"id"`
			Message string `json:"message"`
			Success bool   `json:"success"`
		}{
			id, "game_player_created", true,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gamePlayersCreatedResponse)
	})

	http.HandleFunc("/players/add", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		var requestData struct {
			Name string `json:"name"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"invalid_json", false})
			return
		}

		if requestData.Name == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"validation_error: name can't be blank", false})
			return
		}

		result, err := db.Exec("INSERT INTO players (name) VALUES (?)", requestData.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(Response{"internal_server_error", false})
			return
		}

		id, err := result.LastInsertId()

		userCreatedResponse := struct {
			UserId  int64  `json:"id"`
			Message string `json:"message"`
			Success bool   `json:"success"`
		}{
			id, "user_created", true,
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(userCreatedResponse)
	})

	log.Printf("Server running on port %s\n", PORT)
	log.Fatal(server.ListenAndServe())
}
