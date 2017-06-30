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
    "log"
    "bytes"
    "encoding/json"
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
    r.HandleFunc("/events", readEventHistoryHandler).Methods("GET")
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
        return
    }

    e, err := getEvent(eventId)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    encoder := json.NewEncoder(w)

    w.Header().Set("Content-Type", "application/json")
    err = encoder.Encode(&e)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func readEventHistoryHandler(w http.ResponseWriter, r *http.Request) {
    headId := r.URL.Query().Get("head")
    if headId == "" {
        logger.Println("Invalid value for head.")
        http.Error(w, "Head cannot be empty", http.StatusBadRequest)
        return
    }

    history := make([]*event, 0)

    nextEventId := headId
    for nextEventId != "0000000000000000000000000000000000000000000000000000000000000000" {
        e, err := getEvent(nextEventId)
        if err != nil {
            logger.Println("Error getting event for history.", err.Error())
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }

        history = append(history, e)
        nextEventId = e.Previous
    }

    resp := &historyResp{
        Items: history,
    }

    encoder := json.NewEncoder(w)

    w.Header().Set("Content-Type", "application/json")
    err := encoder.Encode(resp)
    if err != nil {
        logger.Println("Error encoding history response.", err.Error())
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}

type historyResp struct {
    Items []*event `json:"items"`
}

func getEvent(eventId string) (*event, error) {
    logger.Println("Getting event:", eventId)
    res, err := s3svc.GetObject(&s3.GetObjectInput{
        Bucket: aws.String(bucket),
        Key: aws.String(eventId),
    })
    if err != nil {
        logger.Println("Error in GetObject.", err.Error())
        return nil, err
    }

    var buf bytes.Buffer
    buf.ReadFrom(res.Body)
    res.Body.Close()

    logger.Println(buf.String())

    var e event
    err = json.Unmarshal(buf.Bytes(), &e)
    if err != nil {
        logger.Println("Error unmarshalling event.", err.Error())
        return nil, err
    }

    return &e, nil
}

type event struct {
    Previous string `json:"previous"`
    Type string `json:"type"`
    Data string `json:"data"`
}
