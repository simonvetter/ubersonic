package main

import (
    "log"
    "fmt"
    "net/http"
    "time"
    "io"
    "os"
    "encoding/hex"
)

type ApiServer struct {
    logger      *log.Logger
    db          *SubsonicDB
    server      *http.Server
    port        int
    certFile    string
    keyFile     string
}

// creates a subsonic api server
func NewServer(logger *log.Logger, db *SubsonicDB, port int, certFile string, keyFile string) (s *ApiServer) {
    var mux *http.ServeMux

    if logger == nil || db == nil {
        panic("nil logger or db")
    }

    s = &ApiServer{}
    s.logger    = logger
    s.db        = db
    s.port      = port
    s.certFile  = certFile
    s.keyFile   = keyFile

    mux         = http.NewServeMux()
    s.server    = &http.Server{
        Addr:           fmt.Sprintf(":%v", s.port),
        Handler:        mux,
        ErrorLog:       logger,
        ReadTimeout:    10 * time.Second,
        WriteTimeout:   300 * time.Second,
    }

    // funnel all requests to /rest/ to our general handler to take care of
    // auth and routing in a single place.
    mux.HandleFunc("/rest/", s.apiReqHandler)

    return
}

// listens and starts serving requests
func (s *ApiServer) Listen() (err error) {
    if s.certFile != "" && s.keyFile != "" {
        err = s.server.ListenAndServeTLS(s.certFile, s.keyFile)
    } else {
        err = s.server.ListenAndServe()
    }

    return
}

// main request handler
func (s *ApiServer) apiReqHandler(res http.ResponseWriter, req *http.Request) {
    var username    string
    var err         error

    // authenticate requests

    // only allow GET requests, for now
    if req.Method != "GET" {
        http.Error(res, "method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // decode request body, if any
	err = req.ParseForm(); if err != nil {
        http.Error(res, "internal server error", http.StatusInternalServerError)
        s.logger.Print("failed to parse request:", err)
        return
    }

    // verify credentials
    if  len(req.Form["u"]) != 1 || len(req.Form["p"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "credentials missing"))
        return
    } else {
        var password    string
        var ok          bool

        username    = req.Form["u"][0]
        password    = req.Form["p"][0]

        // the subsonic api allows for hex-encoded passwords with the "enc:" prefix
        if len(password) > 4 && password[0:4] == "enc:" {
            var decodedPassword []byte
            decodedPassword, err = hex.DecodeString(password[4:]); if err != nil {
                s.writeSubsonicResponse(res, req, NewSubsonicError(40, "wrong username or password"))
                s.logger.Print("failed to check auth:", err)
                return
            }
            password    = string(decodedPassword)
        }

        ok, err = s.db.CheckPassword(username, password); if err != nil {
            s.writeSubsonicResponse(res, req, NewSubsonicError(40, "wrong username or password"))
            s.logger.Print("failed to check auth:", err)
            return
        }

        // wrong credentials, bail out and log a message
        if !ok {
            s.writeSubsonicResponse(res, req, NewSubsonicError(40, "wrong username or password"))
            s.logger.Printf("auth failure for user '%s'", username)
            return
        }
    }

    // url router
    switch req.URL.Path {
        case "/rest/ping.view":
            s.Ping(res, req)
        case "/rest/getArtists.view":
            s.GetArtists(res, req)
        case "/rest/getArtist.view":
            s.GetArtist(res, req)
        case "/rest/getAlbum.view":
            s.GetAlbum(res, req)
        case "/rest/getSong.view":
            s.GetSong(res, req)
        case "/rest/getCoverArt.view":
            s.GetCoverArt(res, req)
        case "/rest/getIndexes.view":
            s.GetIndexes(res, req)
        case "/rest/stream.view", "/rest/download.view":
            s.Stream(res, req)
        default:
            http.NotFound(res, req)
    }

    s.logger.Printf("%s %s %s %s", req.RemoteAddr, username, req.Method, req.URL.Path)

    return
}

// writes a SubsonicResponse object back to the client, handling errors and setting
// http headers as needed.
func (s *ApiServer) writeSubsonicResponse(res http.ResponseWriter, req *http.Request, sr *SubsonicResponse) (err error) {
    var payload     string

    // serialize our response into a json or xml string depending on the requested encoding
    if len(req.Form["f"]) == 1 && req.Form["f"][0] == "json" {
        res.Header().Set("Content-Type", "text/json")
        payload, err = sr.MarshalJson();
    } else {
        res.Header().Set("Content-Type", "text/xml")
        payload, err = sr.MarshalXml();
    }

    if err != nil {
        s.logger.Print("failed to serialize subsonic response:", err)
        http.Error(res, "internal server error", http.StatusInternalServerError)
    } else {

        // push the xml response through the socket
        _, err = io.WriteString(res, payload); if err != nil {
            s.logger.Print("failed to write http response body:", err)
        }
    }

    return
}

// handles ping.view requests
func (s *ApiServer) Ping(res http.ResponseWriter, req *http.Request) {
    // ping responses are basically just an 'ok' response with an empty payload
    var sr  = NewSubsonicResponse()
    s.writeSubsonicResponse(res, req, sr)

    return
}

// handles getArtists.view requests
func (s *ApiServer) GetArtists(res http.ResponseWriter, req *http.Request) {
    var err error
    var sr  = NewSubsonicResponse()

    sr.Artists, err = s.db.GetIndexedArtists()
    if err != nil {
        s.logger.Print("failed to build indexed artist list:", err)
        s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
    } else {
        s.writeSubsonicResponse(res, req, sr)
    }

    return
}

// handles getArtist.view requests
func (s *ApiServer) GetArtist(res http.ResponseWriter, req *http.Request) {
    var err error
    var id  string

    if len(req.Form["id"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "id parameter missing"))
    } else {
        var sr = NewSubsonicResponse()

        id = req.Form["id"][0]
        sr.Artist, err = s.db.GetArtist(id)
        if err != nil {
            if err == ErrItemNotFound {
                s.writeSubsonicResponse(res, req, NewSubsonicError(70, "not found"))
            } else {
                s.logger.Print("failed to fetch artist:", err)
                s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
            }
        } else {
            s.writeSubsonicResponse(res, req, sr)
        }
    }

    return
}

// handles getCoveArt.view requests
func (s *ApiServer) GetCoverArt(res http.ResponseWriter, req *http.Request) {
    var id          string
    var coverArt    []byte
    var err         error

    if len(req.Form["id"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "id parameter missing"))
    } else {
        id = req.Form["id"][0]
        coverArt, err = s.db.GetCoverArt(id); if err != nil {
            s.logger.Print("failed to get covert art:", err)
            s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
        } else if len(coverArt) > 0 {
            res.Write(coverArt)
        } else {
            s.writeSubsonicResponse(res, req, NewSubsonicError(70, "not found"))
        }
    }

    return
}

// handles getAlbum.view requests
func (s *ApiServer) GetAlbum(res http.ResponseWriter, req *http.Request) {
    var err error
    var id  string
    var sr  = NewSubsonicResponse()

    if len(req.Form["id"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "id parameter missing"))
    } else {
        id = req.Form["id"][0]
        sr.Album, err = s.db.GetAlbum(id)
        if err != nil && err == ErrItemNotFound {
            s.writeSubsonicResponse(res, req, NewSubsonicError(70, "not found"))
        } else if err != nil {
            s.logger.Print("failed to fetch album:", err)
            s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
        } else {
            s.writeSubsonicResponse(res, req, sr)
        }
    }

    return
}

// handle getIndexes.view requests
func (s *ApiServer) GetIndexes(res http.ResponseWriter, req *http.Request) {
    var err error
    var sr  = NewSubsonicResponse()

    sr.Indexes, err = s.db.GetIndexes()
    if err != nil {
        s.logger.Print("failed to build indexes:", err)
        s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
    } else {
        s.writeSubsonicResponse(res, req, sr)
    }

    return
}

// handles getSong.view requests
func (s *ApiServer) GetSong(res http.ResponseWriter, req *http.Request) {
    var err error
    var id  string
    var sr  = NewSubsonicResponse()

    if len(req.Form["id"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "id parameter missing"))
    } else {
        id = req.Form["id"][0]
        sr.Song, err = s.db.GetSong(id)
        if err != nil && err == ErrItemNotFound {
            s.writeSubsonicResponse(res, req, NewSubsonicError(70, "not found"))
        } else if err != nil {
            s.logger.Print("failed to fetch song:", err)
            s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
        } else {
            s.writeSubsonicResponse(res, req, sr)
        }
    }

    return
}

// handles stream.view and download.view requests
func (s *ApiServer) Stream(res http.ResponseWriter, req *http.Request) {
    var err     error
    var id      string
    var ss      *SubsonicSong
    var file    *os.File
    var finfo   os.FileInfo

    if len(req.Form["id"]) != 1 {
        s.writeSubsonicResponse(res, req, NewSubsonicError(10, "id parameter missing"))
    } else {
        id = req.Form["id"][0]
        ss, err = s.db.GetSong(id)
        if (err != nil && err == ErrItemNotFound) || ss.Path == "" {
            s.writeSubsonicResponse(res, req, NewSubsonicError(70, "not found"))
        } else if err != nil {
            s.logger.Print("failed to stream file:", err)
            s.writeSubsonicResponse(res, req, NewSubsonicError(0, "internal server error"))
        } else {
            // stream the file
            file, err = os.Open(ss.Path); if err != nil {
                s.logger.Print("failed to open file:", err)
                s.writeSubsonicResponse(res, req, NewSubsonicError(10, "file not found"))
                return
            }

            // attempt to stat to get file mtime
            finfo, err  = file.Stat(); if err != nil {
                // failing here is not fatal. file mtime is merely used to handle
                // If-Modified headers in requests.
                s.logger.Print("failed to stat file:", err)
                finfo   = nil
            }

            // serve the file.
            http.ServeContent(res, req, ss.Path, finfo.ModTime(), file)

            err = file.Close(); if err != nil {
                s.logger.Print("failed to close file:", err)
            }
        }
    }

    return
}
