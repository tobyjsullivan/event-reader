package main

import (
    "os"

    "fmt"
    "net/http"

    "github.com/urfave/negroni"
    "github.com/gorilla/mux"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/credentials"
    "github.com/aws/aws-sdk-go/service/s3"
    "io"
    "log"
)

var (
    bucket string
    s3svc *s3.S3
    logger *log.Logger
)

func init() {
    region := os.Getenv("AWS_REGION")
    bucket = os.Getenv("S3_BUCKET")

    logger = log.New(os.Stdout, "[svc] ", 0)

    sess := session.Must(session.NewSession(&aws.Config{
        Credentials: credentials.NewEnvCredentials(),
        Region: aws.String(region),
        Logger: aws.LoggerFunc(logger.Println),
        LogLevel: aws.LogLevel(aws.LogOff),
    }))

    s3svc = s3.New(sess)
}

func main() {
    r := buildRoutes()

    n := negroni.New()
    n.UseHandler(r)

    port := os.Getenv("PORT")
    if port == "" {
        port = "3000"
    }

    n.Run(":" + port)
}

func buildRoutes() http.Handler {
    r := mux.NewRouter()
    r.HandleFunc("/", statusHandler).Methods("GET")
    r.HandleFunc("/events/{eventId}", readEventHandler).Methods("GET")

    return r
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprint(w, "The service is online!\n")
}

func readEventHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    eventId := vars["eventId"]

    if eventId == "" {
        http.Error(w, "Event ID cannot be empty", http.StatusBadRequest)
    }

    res, err := s3svc.GetObject(&s3.GetObjectInput{
        Bucket: aws.String(bucket),
        Key: aws.String(eventId),
    })
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    io.Copy(w, res.Body)
}
