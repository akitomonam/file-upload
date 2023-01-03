// main.go
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
	"upload/file"

	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
)

type Config struct {
	DBMS   string `json:"dbms"`
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Server string `json:"server"`
	DBName string `json:"dbname"`
	GoPort string `json:"go_port"`
}

type Users struct {
	ID        int    `db:"id"`
	Username  string `db:"username"`
	Password  string `db:"password"`
	File_path string `db:"file_path"`
}

// Papers ファイルのメタ情報
type Papers struct {
	ID         int
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	Publisher  string    `json:"publisher"`
	Year       int       `json:"year"`
	Abstract   string    `json:"abstract"`
	File_name  string    `json:"file_name"`
	File_path  string    `json:"file_path"`
	User_id    int       `json:"user_id"`
	Created_at time.Time `json:"created_at"`
}

type Favorites struct {
	ID      int `db:"id"`
	Paperid int `db:"paper_id"`
	Userid  int `db:"user_id"`
}

type Keywords struct {
	ID      int    `db:"id"`
	Paperid int    `db:"paper_id"`
	Keyword string `db:"keyword"`
}

type Sessions struct {
	Session_Token string `db:"session_token"`
	User_id       int    `db:"user_id"`
}

var db *gorm.DB
var store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))

func getConfig() Config {
	// 設定ファイルを開く
	file, err := os.Open("db_config.json")
	if err != nil {
		return Config{}
	}
	defer file.Close()

	// 設定ファイルを読み込む
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return Config{}
	}

	// 設定ファイルをパースする
	var config Config
	if err := json.Unmarshal(bytes, &config); err != nil {
		return Config{}
	}

	return config
}

// IndexHandler インデックスページを表示するハンドラ
func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	http.ServeFile(w, r, "index.html")
}

// UploadHandler アップロードを行うハンドラ
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	file.UploadHandler(w, r)
}

// GetTableList テーブル一覧を取得する
func getTableList(db *gorm.DB) ([]string, error) {
	// テーブル一覧を取得するSQL文
	query := "SHOW TABLES"
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 取得したテーブル一覧を格納するスライス
	tables := make([]string, 0)

	// 取得した行を1行ずつ処理する
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}

// DB接続
func sqlConnect(config Config) (database *gorm.DB, err error) {
	DBMS := config.DBMS
	USER := config.User
	PASS := config.Pass
	PROTOCOL := config.Server
	DBNAME := config.DBName

	CONNECT := USER + ":" + PASS + "@" + PROTOCOL + "/" + DBNAME + "?charset=utf8&parseTime=true&loc=Asia%2FTokyo"
	return gorm.Open(DBMS, CONNECT)
}

// APITablesHandler Papersの中身をJSON形式で返すハンドラ
func apiTablesHandler(w http.ResponseWriter, r *http.Request) {
	// CORSのアクセス制御を行う
	w.Header().Set("Access-Control-Allow-Origin", "*")    // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "GET") // GETメソッドのみを許可する

	sessionToken := r.URL.Query().Get("sessionToken")
	fmt.Println("table-list-sessionToken:", sessionToken)

	var fileDbs []Papers
	if sessionToken == "" {
		fmt.Println("セッショントークンなし")
		// Papersからすべてのレコードを取得する
		if err := db.Find(&fileDbs).Error; err != nil {
			// エラーを出力する
			fmt.Println("エラー:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		fmt.Println("セッショントークンあり")
		session, err := FindSession(sessionToken)
		if err != nil {
			fmt.Println("セッショントークンが無効です。:")
			http.Error(w, "セッショントークンが無効です。", http.StatusUnauthorized)
			return
		}

		user_id := session.User_id
		//user_idを使用して、Papersからレコードを取得する
		if err := db.Where("user_id = ?", user_id).Find(&fileDbs).Error; err != nil {
			// エラーを出力する
			fmt.Println("エラー:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Papersの中身をJSON形式で返す
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fileDbs); err != nil {
		fmt.Println("エラー:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")    // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "GET") // GETメソッドのみを許可する
	fmt.Println("deleteFileHandlerが呼び出されました")

	fileID, _ := strconv.Atoi(r.URL.Query().Get("fileId"))
	// 削除対象のファイルを特定し、削除する処理を実行する
	// PapersテーブルからIDを指定してレコードを取得する
	var fileDb Papers
	if err := db.Where("id = ?", fileID).First(&fileDb).Error; err != nil {
		// http.Error(w, "レコードが見つかりません", http.StatusNotFound)
		fmt.Println("レコードが見つかりません")
		fmt.Println("レコードが見つかりません:", err)
		// レスポンスを返す
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"result": "false"})
		return
	}
	// Papersテーブルからレコードを削除する
	fmt.Println("filedb:", &fileDb)
	if err := db.Delete(&fileDb).Error; err != nil {
		http.Error(w, "データの削除に失敗しました", http.StatusInternalServerError)
		fmt.Println("データの削除に失敗しました")
		return
	}
	//DBから取得したファイルパスでファイルの削除
	if err := os.Remove(fileDb.File_path); err != nil {
		http.Error(w, "ファイルの削除に失敗しました", http.StatusInternalServerError)
		fmt.Println("ファイルの削除に失敗しました")
		return
	}
	// レスポンスを返す
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"result": "true"})
	fmt.Println("finish deletefile")
}

// pdfのプレビュー処理
func apiPreviewHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("apiPreviewHandlerが呼び出されました")
	// CORSのアクセス制御を行う
	w.Header().Set("Access-Control-Allow-Origin", "*")    // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "GET") // GETメソッドのみを許可する

	// ファイル名を取得する
	fileId, err := strconv.Atoi(r.URL.Query().Get("fileId"))
	fmt.Println("クリックしたファイルのID:", fileId)

	// ファイルのURLを取得する
	fileUrl, err := getFileUrl(fileId)
	fmt.Println("fileUrl:", fileUrl)
	if err != nil {
		// エラーを出力する
		fmt.Println("エラー:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		// レスポンスボディを作成する
		responseBody, err := json.Marshal(map[string]string{
			"fileUrl": fileUrl,
		})
		if err != nil {
			// エラーを出力する
			fmt.Println("エラー:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// レスポンスを送信する
		w.Header().Set("Content-Type", "application/json")
		w.Write(responseBody)
	}
}

// ファイルのURLを取得する関数
func getFileUrl(fileId int) (string, error) {
	// ファイルのURLを格納する変数
	var fileUrl string

	// Papersから指定したファイル名のレコードを1件取得する
	var fileDb Papers
	if err := db.Where("id = ?", fileId).First(&fileDb).Error; err != nil {
		fmt.Println("エラー(getFileUrl):", err)
		return "", err
	}

	// 取得したレコードのfilepathをfileUrlに代入する
	fileUrl = fileDb.File_path

	return fileUrl, nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("loginHandlerが呼び出されました")
	w.Header().Set("Access-Control-Allow-Origin", "*")             // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "POST")         // POSTメソッドのみを許可する
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type") // Content-Typeヘッダーのみを許可する
	username := r.FormValue("username")
	password := r.FormValue("password")

	// ユーザー名とパスワードを使用して、ログイン処理を行う
	var user Users
	if err := db.Where("username = ? and password = ?", username, password).First(&user).Error; err != nil {
		// ログインに失敗した場合
		if gorm.IsRecordNotFoundError(err) {
			// ユーザー名またはパスワードが間違っている
			resp := struct {
				Success bool   `json:"success"`
				Message string `json:"message"`
			}{
				Success: false,
				Message: "ユーザー名またはパスワードが間違っています",
			}
			respBytes, _ := json.Marshal(resp)
			w.Write(respBytes)
			return
		}
	}
	session, err := store.Get(r, "session")
	if err != nil {
		fmt.Println("セッションGET失敗")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// トークンを生成する
	b := make([]byte, 32)
	rand.Read(b)
	token := base64.StdEncoding.EncodeToString(b)
	// セッションにトークンを保存する(必要ないかも)
	session.Values["token"] = token
	session.Save(r, w)

	user_id := user.ID

	if err := db.Create(&Sessions{User_id: user_id, Session_Token: token}).Error; err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{
			Success: false,
			Message: "セッション取得に失敗しました。",
		}
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
		return
	}

	resp := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Token   string `json:"token"`
	}{
		Success: true,
		Message: "ログインに成功しました",
		Token:   token,
	}
	respBytes, _ := json.Marshal(resp)
	w.Write(respBytes)
}

func signupHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("signupHandlerが呼び出されました")
	w.Header().Set("Access-Control-Allow-Origin", "*")             // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "POST")         // POSTメソッドのみを許可する
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type") // Content-Typeヘッダーのみを許可する
	username := r.FormValue("username")
	password := r.FormValue("password")

	// ユーザー名とパスワードを使用して、サインアップ処理を行う
	//ユーザー名とパスワードをDBに保存する
	if err := db.Create(&Users{Username: username, Password: password}).Error; err != nil {
		resp := struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}{
			Success: false,
			Message: "既に登録されているユーザー名です",
		}
		respBytes, _ := json.Marshal(resp)
		w.Write(respBytes)
		return
	}

	resp := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: true,
		Message: "サインアップに成功しました",
	}
	respBytes, _ := json.Marshal(resp)
	w.Write(respBytes)
}

func userinfoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")    // 任意のドメインからのアクセスを許可する
	w.Header().Set("Access-Control-Allow-Methods", "GET") // GETメソッドのみを許可する

	sessionToken := r.URL.Query().Get("sessionToken")
	fmt.Println("sessionToken:", sessionToken)

	session, err := FindSession(sessionToken)
	if err != nil {
		fmt.Println("セッショントークンが無効です。:")
		http.Error(w, "セッショントークンが無効です。", http.StatusUnauthorized)
		return
	}

	user_id := session.User_id

	var userDb Users
	if err := db.Where("id = ?", user_id).First(&userDb).Error; err != nil {
		fmt.Println("エラー(useInfo):", err)
		return
	}

	// ユーザー情報を必要な情報のみ返す
	userInfo := Users{
		Username:  userDb.Username,
		File_path: userDb.File_path,
	}

	// ユーザー情報をすべて返す
	res, err := json.Marshal(userInfo)
	if err != nil {
		fmt.Println("エラー(useInfo):", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}

func FindSession(sessionToken string) (Sessions, error) {
	// セッションDBを検索
	var session Sessions
	err := db.Where("session_token = ?", sessionToken).First(&session).Error
	if err != nil {
		// セッションが見つからない場合や、エラーが発生した場合
		return Sessions{}, fmt.Errorf("セッションが無効です: %v", err)
	}
	return session, nil
}

func setupRoutes(config Config) {
	mux := http.NewServeMux()
	// mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/upload/file", uploadHandler)
	mux.HandleFunc("/api/delete", deleteFileHandler)
	mux.HandleFunc("/api/tables", apiTablesHandler)
	mux.HandleFunc("/api/preview", apiPreviewHandler)
	mux.HandleFunc("/api/login", loginHandler)
	mux.HandleFunc("/api/signup", signupHandler)
	mux.HandleFunc("/api/userinfo", userinfoHandler)
	mux.Handle("/uploadfiles/", http.StripPrefix("/uploadfiles/", http.FileServer(http.Dir("./uploadfiles"))))

	if err := http.ListenAndServe(config.GoPort, mux); err != nil {
		log.Fatal(err)
	}
}

func serveVueApp() {
	fs := http.FileServer(http.Dir("./fronted/dist"))
	fmt.Println("Vue.jsアプリケーションをサーブ完了")
	http.Handle("/", fs)
}

func main() {
	// 設定値を取得する
	config := getConfig()
	// データベースと接続
	var err error
	db, err = sqlConnect(config)
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	} else {
		fmt.Println("DB接続成功")
	}
	defer db.Close()

	setupRoutes(config)
	serveVueApp()
}
