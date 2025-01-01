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
		max_points INTEGER NOT NULL
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
	ID        int    `json:"id"`
	MaxPoints string `json:"max_points"`
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

func openLogFile(logfile string) {
	if logfile != "" {
		lf, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
		if err != nil {
			log.Fatal("OpenLogfile: os.OpenFile:", err)
		}
		log.SetOutput(lf)
	}
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {
	logPath := "development.log"
	if PORT == "" {
		PORT = "8080"
	}

	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	openLogFile(logPath)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Response{"all good", true})
	})

	mux.HandleFunc("/games/new", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		var requestData struct {
			MaxPoints int `json:"max_points"`
		}

		err := json.NewDecoder(r.Body).Decode(&requestData)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(Response{"invalid_json", false})
			return
		}

		result, err := db.Exec("INSERT INTO games (max_points) VALUES (?)", requestData.MaxPoints)
		if err != nil {
			respondWithError(w, 500)
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			respondWithError(w, 500)
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

	mux.HandleFunc("/games", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(Response{"method_not_allowed", false})
			return
		}

		id := r.URL.Query().Get("game_id")

		rows, err := db.Query("SELECT game_id, player_id, score FROM game_players WHERE game_id = ?", id)
		if err != nil {
			respondWithError(w, 500)
			return
		}
		defer rows.Close()

		gamePlayers := make([]GamePlayer, 0)

		for rows.Next() {
			gamePlayer := new(GamePlayer)
			err := rows.Scan(&gamePlayer.GameId, &gamePlayer.PlayerId, &gamePlayer.Score)
			if err != nil {
				respondWithError(w, 500)
				return
			}
			gamePlayers = append(gamePlayers, *gamePlayer)
		}

		if err = rows.Err(); err != nil {
			respondWithError(w, 500)
			return
		}

		var gameResponse = struct {
			Data []GamePlayer `json:"data"`
		}{gamePlayers}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(gameResponse)
	})

	mux.HandleFunc("/games/update-score", func(w http.ResponseWriter, r *http.Request) {
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
			respondWithError(w, 500)
			return
		}

		row := db.QueryRow("SELECT score FROM game_players WHERE game_id = ? AND player_id = ?", requestData.GameId, requestData.PlayerId)
		gamePlayer := new(GamePlayer)

		err = row.Scan(&gamePlayer.Score)
		if err != nil {
			respondWithError(w, 500)
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

	mux.HandleFunc("/games/players/add", func(w http.ResponseWriter, r *http.Request) {
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
			respondWithError(w, 500)
			return
		}

		id, err := result.LastInsertId()
		if err != nil {
			respondWithError(w, 500)
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

	mux.HandleFunc("/players/add", func(w http.ResponseWriter, r *http.Request) {
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
			respondWithError(w, 500)
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

	middlewares := serverMiddlewares(mux, corsMiddleware, logRequest)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", PORT),
		Handler: middlewares,
	}

	log.Printf("Server running on port %s\n", PORT)
	log.Fatal(server.ListenAndServe())
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func serverMiddlewares(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, middleware := range middlewares {
		handler = middleware(handler)
	}
	return handler
}

func respondWithError(w http.ResponseWriter, statusCode int) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Message: "internal_server_error", Success: false})
}
