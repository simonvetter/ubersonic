package main

import (
    "database/sql"
    "fmt"
    "strings"
    "errors"
    "path"
    "mime"

    _ "github.com/mattn/go-sqlite3"
)

var ErrItemNotFound = errors.New("not found")

type SubsonicDB struct {
    db          *sql.DB
    dbpath      string
    statements  []*sql.Stmt
}

const (
    GETARTISTS      = iota
    GETARTIST       = iota
    GETARTISTALBUMS = iota
    GETARTISTART    = iota
    GETALBUMCNT     = iota
    GETSONGCNT      = iota
    GETALBUM        = iota
    GETALBUMART     = iota
    GETALBUMSONGS   = iota
    GETSONG         = iota
    GETPASSWORD     = iota
    GETMTIME        = iota
)

// creates a new SubsonicDB store
func NewSubsonicDB(dbpath string) (sdb *SubsonicDB) {
    sdb = &SubsonicDB{
        dbpath:         dbpath,
        statements:     make([]*sql.Stmt, 12),
    }

    return
}

// opens the sqlite database and prepares statements
func (sdb *SubsonicDB) Open() (err error) {
    var db      *sql.DB
    var stmt    *sql.Stmt

    // attempt to open the database
    db, err  = sql.Open("sqlite3", fmt.Sprintf("file://%s?mode=ro", sdb.dbpath))
    if err != nil {
        return
    }

    // prepare sql statements
    // getArtists
    stmt, err = db.Prepare(`SELECT id, name FROM artists ORDER BY name COLLATE NOCASE ASC`)
    if err != nil {
        return
    }
    sdb.statements[GETARTISTS] = stmt

    stmt, err = db.Prepare(`SELECT id, name FROM artists WHERE id=?`)
    if err != nil {
        return
    }
    sdb.statements[GETARTIST] = stmt

    // get artist albums
    stmt, err = db.Prepare(`SELECT id, title, artistid, artist FROM albums
                            WHERE artistid=?`)
    if err != nil {
        return
    }
    sdb.statements[GETARTISTALBUMS] = stmt

    // get album 
    stmt, err = db.Prepare(`SELECT id, title, artistid, artist FROM albums
                            WHERE id=?`)
    if err != nil {
        return
    }
    sdb.statements[GETALBUM] = stmt

    // get album count for artist
    stmt, err = db.Prepare(`SELECT count(id) FROM albums WHERE artistid=?`)
    if err != nil {
        return
    }
    sdb.statements[GETALBUMCNT] = stmt

    // get song count for album
    stmt, err = db.Prepare(`SELECT count(id) FROM songs WHERE artistid=?
                            AND albumid=?`)
    if err != nil {
        return
    }
    sdb.statements[GETSONGCNT] = stmt

    // get album art for artist
    stmt, err = db.Prepare(`SELECT image FROM covers WHERE artistId=?
                            AND image NOT NULL`)
    if err != nil {
        return
    }
    sdb.statements[GETARTISTART] = stmt

    // get album art
    stmt, err = db.Prepare(`SELECT image FROM covers WHERE albumId=?
                             AND image NOT NULL`)
    if err != nil {
        return
    }
    sdb.statements[GETALBUMART] = stmt

    // get songs for album
    stmt, err = db.Prepare(`SELECT id, title, albumid, album, artistid, artist,
                            trackn, discn, year, duration, bitRate, genre, type,
                            filename
                            FROM songs WHERE artistid=? AND albumid=?`)
    if err != nil {
        return
    }
    sdb.statements[GETALBUMSONGS] = stmt

    // get song
    stmt, err = db.Prepare(`SELECT id, title, albumid, album, artistid, artist,
                            trackn, discn, year, duration, bitRate, genre, type,
                            filename
                            FROM songs WHERE id=?`)
    if err != nil {
        return
    }
    sdb.statements[GETSONG] = stmt

    // get user
    stmt, err = db.Prepare(`SELECT count() FROM users WHERE username=? AND password=?`)
    if err != nil {
        return
    }
    sdb.statements[GETPASSWORD] = stmt

    // get mtime
    stmt, err = db.Prepare(`SELECT mtime FROM last_update_ts WHERE table_name=?`)
    if err != nil {
        return
    }
    sdb.statements[GETMTIME] = stmt

    return
}

// returns a list of indexed artists with per-artist album count
func (sdb *SubsonicDB) GetIndexedArtists() (indexesptr *[]*SubsonicIndex, err error) {
    var si      *SubsonicIndex
    var rows    *sql.Rows
    var indexes = make([]*SubsonicIndex, 0)

    // run the prepared get artist statement
    rows, err = sdb.statements[GETARTISTS].Query(); if err != nil {
        return
    }

    // iterate through all results
    for rows.Next() {
        var id      uint64
        var name    string
        var artist  *SubsonicArtist

        err = rows.Scan(&id, &name); if err != nil {
            return
        }

        if name == "" {
            continue
        }

        // create an Artist object and add it to the index
        artist = &SubsonicArtist{
            Id:         id,
            Name:       name,
            CoverArt:   fmt.Sprintf("ar-%v", id),
        }

        // count all albums for this artist
        err = sdb.statements[GETALBUMCNT].QueryRow(artist.Id).Scan(&artist.AlbumCount)
        if err != nil {
            return
        }

        // make sure this artist belongs to the current index.
        // if not, add the old one to the list and create a new one
        if si == nil || strings.ToUpper(name)[0] != si.Name[0] {
            if si != nil {
                indexes = append(indexes, si)
            }

            si = &SubsonicIndex{
                Name:       strings.ToUpper(string(name[0])),
                Artists:    make([]*SubsonicArtist, 0),
            }
        }

        // add the artist to the index
        si.Artists = append(si.Artists, artist)
    }

    // add the last index we were working on to the list, if any
    if si != nil {
        indexes = append(indexes, si)
    }

    indexesptr  = &indexes
    return
}

// returns a populated SubsonicIndexes object
func (sdb *SubsonicDB) GetIndexes() (indexes *SubsonicIndexes, err error) {
    var lastUpdated uint64
    var idx         *[]*SubsonicIndex;

    // get the last update timestamp
    err = sdb.statements[GETMTIME].QueryRow("songs").Scan(&lastUpdated)
    if err != nil {
        return
    }

    // get a list of indexes
    idx, err = sdb.GetIndexedArtists();
    if err != nil {
        return
    }

    // attach the timestamp and the list of indexes to a SubsonicIndexes object
    indexes = &SubsonicIndexes{
        LastModified:   lastUpdated,
        Index:          idx,
    }

    return
}

// returns a cover art image for the specified artist, album or song
func (sdb *SubsonicDB) GetCoverArt(id string) (coverArt []byte, err error) {
    var rows    *sql.Rows

    if len(id) < 4 {
        return
    }

    switch id[0:3] {
        case "ar-":
            rows, err = sdb.statements[GETARTISTART].Query(id[3:]); if err != nil {
                return
            }
        case "al-":
            rows, err = sdb.statements[GETALBUMART].Query(id[3:]); if err != nil {
                return
            }
        default:
            return
    }

    // walk through all results, pick the first non-empty image
    for rows.Next() {
        err = rows.Scan(&coverArt); if err != nil {
            rows.Close()
            return
        }

        if len(coverArt) > 0 {
            rows.Close()
            return
        }
    }

    return
}

// returns a list of all albums for a given artist id
func (sdb *SubsonicDB) GetArtist(id string) (sr *SubsonicArtist, err error) {
    var rows    *sql.Rows
    var albums  = make([]*SubsonicAlbum, 0)
    sr = &SubsonicArtist{}

    // fetch the artist name and id
    // this also makes sure it exists
    err = sdb.statements[GETARTIST].QueryRow(id).Scan(&sr.Id, &sr.Name)
    if err != nil {
        if err == sql.ErrNoRows {
            err = ErrItemNotFound
        }
        return
    }

    sr.CoverArt = fmt.Sprintf("ar-%s", id)

    // fetch all albums for this artist
    rows, err = sdb.statements[GETARTISTALBUMS].Query(id); if err != nil {
        return
    }

    // iterate through each album
    for rows.Next() {
        var sa  *SubsonicAlbum

        sa, err  = sdb.scanAlbum(rows); if err != nil {
            return
        }

        albums = append(albums, sa)

        // keep track of album count
        sr.AlbumCount++
    }

    sr.Albums   = albums

    return
}

// returns a list of all songs for a given album id
func (sdb *SubsonicDB) GetAlbum(id string) (sr *SubsonicAlbum, err error) {
    var rows    *sql.Rows
    var songs   = make([]*SubsonicSong, 0)

    sr  = &SubsonicAlbum{
        CoverArt:   fmt.Sprintf("al-%v", id),
    }

    // fetch the album first
    err = sdb.statements[GETALBUM].QueryRow(id).Scan(
            &sr.Id, &sr.Name, &sr.ArtistId, &sr.ArtistName)
    if err != nil {
        if err == sql.ErrNoRows {
            err = ErrItemNotFound
        }
        return
    }

    // fetch all songs for this album
    rows, err = sdb.statements[GETALBUMSONGS].Query(sr.ArtistId, sr.Id); if err != nil {
        return
    }

    // iterate through album songs
    for rows.Next() {
        var s   *SubsonicSong

        s, err  = scanSong(rows); if err != nil {
            return
        }

        songs       = append(songs, s)
        sr.Duration += s.Duration
        sr.SongCount++
    }

    sr.Songs    = &songs
    return
}

// returns a single song
func (sdb *SubsonicDB) GetSong(id string) (ss *SubsonicSong, err error) {
    var rows    *sql.Rows

    // fetch the song
    rows, err = sdb.statements[GETSONG].Query(id)
    if err != nil {
        if err == sql.ErrNoRows {
            err = ErrItemNotFound
        }
        return
    }

    // we're only expecting one row here
    if rows.Next() {
        ss, err = scanSong(rows); if err != nil {
            return
        }
    } else {
        err = ErrItemNotFound
    }

    rows.Close()
    return
}

// returns ok if the given username and password pair exists in the database
func (sdb *SubsonicDB) CheckPassword(username string, password string) (ok bool, err error) {
    var count   int

    ok  = false

    err = sdb.statements[GETPASSWORD].QueryRow(username, password).Scan(&count); if err != nil {
        return
    }

    // found exactly one match for user,pass in the databse, return ok
    if count == 1 {
        ok = true
    }

    return
}

func scanSong(row *sql.Rows) (s *SubsonicSong, err error) {
    var year        uint64
    var discn       uint64
    var filepath    string
    var filesuffix  string
    s               = &SubsonicSong{}

    err = row.Scan(
            &s.Id, &s.Title, &s.AlbumId, &s.Album, &s.ArtistId, &s.Artist,
            &s.TrackNo, &discn, &year, &s.Duration, &s.BitRate, &s.Genre, &s.Type,
            &filepath); if err != nil {
        return
    }
    // unused
    _   = discn

    filesuffix  = strings.ToLower(path.Ext(filepath))
    if filesuffix != "" {
        // get rid of trailing slash
        s.Suffix        = filesuffix[1:]
        // get mime type
        s.ContentType   = mime.TypeByExtension(filesuffix)
    }

    s.Parent    = s.AlbumId
    s.CoverArt  = fmt.Sprintf("al-%v", s.AlbumId)
    s.Created   = fmt.Sprintf("%04d-01-01T00:00:00", year)
    s.Path      = filepath

    return
}

func (sdb *SubsonicDB) scanAlbum(row *sql.Rows) (sa *SubsonicAlbum, err error) {
    var id          uint64
    var title       string
    var artistid    uint64
    var artist      string
    var songCount   uint64

    err = row.Scan(&id, &title, &artistid, &artist); if err != nil {
        return
    }

    // count the number of songs in that album
    err = sdb.statements[GETSONGCNT].QueryRow(artistid, id).Scan(&songCount)
    if err != nil {
        return
    }

    sa = &SubsonicAlbum{
        Id:         id,
        Name:       title,
        ArtistId:   artistid,
        ArtistName: artist,
        SongCount:  songCount,
        CoverArt:   fmt.Sprintf("al-%v", id),
    }

    return
}
