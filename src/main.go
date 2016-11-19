package main

import (
    "log"
    "os"
    "flag"
)

func main() {
    var logger      *log.Logger = log.New(os.Stdout, "", log.LstdFlags)
    var sdb         *SubsonicDB
    var err         error
    var server      *ApiServer
    var port        int
    var cert    string
    var key     string
    var notls       bool
    var dbPath      string

    // parse command line args
    flag.IntVar(&port, "port", 4001, "port to listen on")
    flag.StringVar(&dbPath, "db", "", "path to sqlite database")
    flag.StringVar(&cert, "cert", "", "path to tls cert file")
    flag.StringVar(&key, "key", "", "path to tls private key")
    flag.BoolVar(&notls, "notls", false, "disable tls, use plain text")
    flag.Parse()

    if dbPath == "" {
        logger.Fatal("need an sqlite database path, please use --db")
    }

    // use tls, it's good for you. letsencrypt is free and dead easy to use :)
    if notls {
        logger.Print(
            "warn: tls is disabled. All traffic (including usernames and passwords) " +
            "will transit in the clear.")
    } else {
        if cert == "" {
            logger.Fatal("please use --cert to specify a tls cert file or --notls")
        }
        if key == "" {
            logger.Fatal("please use --key to specify a tls key file or --notls")
        }
    }

    // open the database
    sdb = NewSubsonicDB(dbPath)
    err = sdb.Open(); if err != nil {
        logger.Fatal("failed to open database:", err)
    }

    server = NewServer(logger, sdb, port, cert, key)

    logger.Printf("starting server on port %v", port)

    err = server.Listen(); if err != nil {
        logger.Fatal("failed to start server:", err)
    }

    return
}
