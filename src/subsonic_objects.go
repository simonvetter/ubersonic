// borrowed from https://github.com/jeena/sonicmonkey/, credits to jeena
package main

import (
    "encoding/xml"
    "encoding/json"
)

type SubsonicResponse struct {
    XMLName         xml.Name                `xml:"subsonic-response"    json:"-"`
    Status          string                  `xml:"status,attr"          json:"status"`
    Version         string                  `xml:"version,attr"         json:"version"`
    Xmlns           string                  `xml:"xmlns,attr"           json:"xmlns"`
    Error           *SubsonicError          `xml:"error"                json:"error,omitempty"`
    AlbumList2      *[]*SubsonicAlbum       `xml:"albumList2>album"     json:"album,omitempty"`
    Album           *SubsonicAlbum          `xml:"album"                json:"album,omitempty"`
    Song            *SubsonicSong           `xml:"song"                 json:"song,omitempty"`
    Indexes         *SubsonicIndexes        `xml:"indexes"              json:"indexes,omitempty"`
    Artists         *[]*SubsonicIndex       `xml:"artists>index"        json:"artists,omitempty"`
    Artist          *SubsonicArtist         `xml:"artist"               json:"artist,omitempty"`
    MusicDirectory  *SubsonicDirectory      `xml:"directory"            json:"directory,omitempty"`
    RandomSongs     *SubsonicRandomSongs    `xml:"randomSongs"          json:"randomSongs,omitempty"`
}

type SubsonicError struct {
    XMLName         xml.Name                `xml:"error"                json:"-"`
    Code            uint64                  `xml:"code,attr"            json:"code"`
    Message         string                  `xml:"message,attr"         json:"message"`
}

type SubsonicAlbum struct {
    XMLName         xml.Name                `xml:"album"                json:"-"`
    Id              uint64                  `xml:"id,attr"              json:"id"`
    Name            string                  `xml:"name,attr"            json:"name"`
    ArtistId        uint64                  `xml:"artistId,attr"        json:"artistId"`
    ArtistName      string                  `xml:"artist,attr"          json:"artist"`
    SongCount       uint64                  `xml:"songCount,attr"       json:"songCount"`
    Duration        uint64                  `xml:"duration,attr"        json:"duration"`
    CoverArt        string                  `xml:"coverArt,attr"        json:"coverArt"`
    Created         string                  `xml:"created,attr"         json:"created"`
    Songs           *[]*SubsonicSong        `xml:"song"                 json:"song,omitempty"`
}

type SubsonicRandomSongs struct {
    XMLName         xml.Name                `xml:"randomSongs"          json:"-"`
    Songs           []*SubsonicSong         `xml:"song"                 json:"song"`
}

type SubsonicSong struct {
    XMLName         xml.Name                `xml:"song"                 json:"-"`
    Id              uint64                  `xml:"id,attr"              json:"id"`
    Parent          uint64                  `xml:"parent,attr"          json:"parent"`
    Title           string                  `xml:"title,attr"           json:"title"`
    Album           string                  `xml:"album,attr"           json:"album"`
    Artist          string                  `xml:"artist,attr"          json:"artist"`
    IsDir           bool                    `xml:"isDir,attr"           json:"isDir"`
    CoverArt        string                  `xml:"coverArt,attr"        json:"coverArt"`
    Created         string                  `xml:"created,attr"         json:"created"`
    Duration        uint64                  `xml:"duration,attr"        json:"duration"`
    Genre           string                  `xml:"genre,attr"           json:"genre"`
    BitRate         uint64                  `xml:"bitRate,attr"         json:"bitRate"`
    Size            uint64                  `xml:"size,attr"            json:"size"`
    Suffix          string                  `xml:"suffix,attr"          json:"suffix"`
    ContentType     string                  `xml:"contentType,attr"     json:"contentType"`
    IsVideo         bool                    `xml:"isVideo,attr"         json:"isVideo"`
    Path            string                  `xml:"path,attr"            json:"path"`
    AlbumId         uint64                  `xml:"albumId,attr"         json:"albumId"`
    ArtistId        uint64                  `xml:"artistId,attr"        json:"artistId"`
    TrackNo         uint64                  `xml:"track,attr"           json:"track"`
    Type            string                  `xml:"type,attr"            json:"type"`
}

type SubsonicArtist struct {
    XMLName         xml.Name                `xml:"artist"               json:"-"`
    Id              uint64                  `xml:"id,attr"              json:"id"`
    Name            string                  `xml:"name,attr"            json:"name"`
    CoverArt        string                  `xml:"coverArt,attr"        json:"coverArt"`
    AlbumCount      uint64                  `xml:"albumCount,attr"      json:"albumCount"`
    Albums          []*SubsonicAlbum        `xml:"album,omitempty"      json:"album,omitempty"`
}

type SubsonicIndexes struct {
    LastModified    uint64                  `xml:"lastModified,attr"    json:"lastModified"`
    Index           *[]*SubsonicIndex       `xml:"index"                json:"index"`
}

type SubsonicIndex struct {
    XMLName         xml.Name                `xml:"index"                json:"-"`
    Name            string                  `xml:"name,attr"            json:"name"`
    Artists         []*SubsonicArtist       `xml:"artist"               json:"artist"`
}

type SubsonicDirectory struct {
    XMLName         xml.Name                `xml:"directory"            json:"-"`
    Id              string                  `xml:"id,attr"              json:"id"`
    Parent          string                  `xml:"parent,attr"          json:"parent"`
    Name            string                  `xml:"name,attr"            json:"name"`
    Starred         string                  `xml:"starred,attr,omitempty"   json:"starred,omitempty"`
    Children        []SubsonicChild         `xml:"child"                json:"child"`
}

type SubsonicChild struct {
    XMLName         xml.Name                `xml:child`
    Id              string                  `xml:"id,attr"`
    Parent          string                  `xml:"parent,attr"`
    Title           string                  `xml:"title,attr"`
    IsDir           bool                    `xml:"isDir,attr"`
    Album           string                  `xml:"album,attr,omitempty"`
    Artist          string                  `xml:"artist,attr,omitempty"`
    Track           uint64                  `xml:"track,attr,omitempty"`
    Year            uint64                  `xml:"year,attr,omitempty"`
    Genre           string                  `xml:"genre,attr,omitempty"`
    CoverArt        uint64                  `xml:"coverart,attr"`
    Size            uint64                  `xml:"size,attr,omitempty"`
    ContentType     string                  `xml:"contentType,attr,omitempty"`
    Suffix          string                  `xml:"suffix,attr,omitempty"`
    Duration        uint64                  `xml:"duration,attr,omitempty"`
    BitRate         uint64                  `xml:"bitRate,attr,omitempty"`
    Path            string                  `xml:"path,attr,omitempty"`
}

// creates an empty, ok SubsonicResponse object
func NewSubsonicResponse() (sr *SubsonicResponse) {
    sr = &SubsonicResponse{
            Status: "ok",
            Xmlns: "http://subsonic.org/restapi",
            Version: "1.10.0",
    }

    return
}

// creates a Subsonic response with an embedded error
func NewSubsonicError(code uint64, message string) (sr *SubsonicResponse) {
    sr          = NewSubsonicResponse()

    sr.Status   = "failed"
    sr.Error    = &SubsonicError{
        Code:       code,
        Message:    message,
    }

    return
}

// turns a SubsonicResponse object into a json-encoded string
func (sr *SubsonicResponse) MarshalJson() (payload string, err error) {
    var b []byte

    b, err = json.MarshalIndent(sr, "", "  ")
    if err == nil {
        payload = string(b)
    }

    return
}

// turns a SubsonicResponse object into an xml-encoded string
func (sr *SubsonicResponse) MarshalXml() (payload string, err error) {
    var b []byte

    b, err = xml.MarshalIndent(sr, "", "  ")
    if err == nil {
        payload = string(b)
    }

    return
}
