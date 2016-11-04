/* I borrowed this file from https://github.com/davidgfnet/supersonic-cpp/
 * credits to David G.F.
 */

#include <cstdlib>
#include <string>
#include <iostream>
#include <vector>
#include <thread>
#include <algorithm>
#include <map>
#include <string.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <dirent.h>
#include <stdio.h>
#include <string>

#include <sqlite3.h>
#include <taglib/fileref.h>
#include <taglib/tag.h>
#include <taglib/tpropertymap.h>
#include <taglib/mpegfile.h>
#include <taglib/id3v2tag.h>
#include <taglib/oggfile.h>
#include <taglib/vorbisfile.h>
#include <taglib/flacfile.h>
#include <taglib/mp4file.h>
#include <taglib/attachedpictureframe.h>
#include <taglib/flacpicture.h>

using namespace std;

const char * init_sql = "\
    CREATE TABLE `albums` (\
        `id`    INTEGER NOT NULL UNIQUE,\
        `title`    TEXT,\
        `artistid`    INTEGER,\
        `artist`    TEXT,\
        `cover`    BLOB,\
        PRIMARY KEY(id)\
    );\
    CREATE TABLE `artists` (\
        `id`    INTEGER NOT NULL UNIQUE,\
        `name`    TEXT,\
        PRIMARY KEY(id)\
    );\
    CREATE TABLE `songs` (\
        `id`    INTEGER NOT NULL UNIQUE,\
        `title`    TEXT,\
        `albumid`    INTEGER,\
        `album`    TEXT,\
        `artistid`    INTEGER,\
        `artist`    TEXT,\
        `trackn`    INTEGER,\
        `discn`    INTEGER,\
        `year`    INTEGER,\
        `duration`    INTEGER,\
        `bitRate`    INTEGER,\
        `genre`    TEXT,\
        `type`    TEXT,\
        `filename`    TEXT,\
        PRIMARY KEY(id)\
    );\
    CREATE TABLE `users` (\
        `username`    TEXT NOT NULL UNIQUE,\
        `password`    TEXT,\
        PRIMARY KEY(username)\
    );\
";

void panic_if(bool cond, string text) {
    if (cond) {
        cerr << text << endl;
        exit(1);
    }
}

// trim spaces from both ends
std::string trim(std::string s) {
    s.erase(s.begin(), std::find_if(s.begin(), s.end(),
            std::not1(std::ptr_fun<int, int>(std::isspace))));

    s.erase(std::find_if(s.rbegin(), s.rend(),
            std::not1(std::ptr_fun<int, int>(std::isspace))).base(), s.end());

    return s;
}

uint64_t calcId(string s) {
    // 64 bit FNV-1a
    uint64_t hash = 14695981039346656037ULL;
    for (unsigned char c: s) {
        hash ^= c;
        hash *= 1099511628211ULL;
    }

    // Sqlite doesn't like unsigned numbers :D
    return hash & 0x7FFFFFFFFFFFFFFF;
}

string base64Decode(const string & input) {
    if (input.length() % 4)
        return "";

    //Setup a vector to hold the result
    string ret;
    unsigned int temp = 0;
    for (unsigned cursor = 0; cursor < input.size(); ) {
        for (unsigned i = 0; i < 4; i++) {
            unsigned char c = *(unsigned char*)&input[cursor];
            temp <<= 6;
            if       (c >= 0x41 && c <= 0x5A)
                temp |= c - 0x41;
            else if  (c >= 0x61 && c <= 0x7A)
                temp |= c - 0x47;
            else if  (c >= 0x30 && c <= 0x39)
                temp |= c + 0x04;
            else if  (c == 0x2B)
                temp |= 0x3E;
            else if  (c == 0x2F)
                temp |= 0x3F;
            else if  (c == '=') {
                if (input.size() - cursor == 1) {
                    ret.push_back((temp >> 16) & 0x000000FF);
                    ret.push_back((temp >> 8 ) & 0x000000FF);
                    return ret;
                }
                else if (input.size() - cursor == 2) {
                    ret.push_back((temp >> 10) & 0x000000FF);
                    return ret;
                }
            }
            cursor++;
        }
        ret.push_back((temp >> 16) & 0x000000FF);
        ret.push_back((temp >> 8 ) & 0x000000FF);
        ret.push_back((temp      ) & 0x000000FF);
    }
    return ret;
}

string getFileExtension(const string& fileName)
{
    string::size_type dot   = fileName.find_last_of(".");
    string::size_type slash = fileName.find_last_of("/");

    if(dot != string::npos && ((slash != string::npos && slash < dot) || slash == string::npos)) {
        return fileName.substr(dot + 1);
    } else {
        return "";
    }
}

void insert_artist(sqlite3 * sqldb, string artist) {
    sqlite3_stmt *stmt;
    sqlite3_prepare_v2(sqldb, "INSERT OR REPLACE INTO `artists` (`id`, `name`) VALUES (?,?);", -1, &stmt, NULL);

    sqlite3_bind_int64(stmt, 1, calcId(artist));
    sqlite3_bind_text (stmt, 2, artist.c_str(), -1, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to insert artist: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);
}

void insert_album(sqlite3 * sqldb, string album, string artist, string cover) {
    sqlite3_stmt    *stmt;
    uint64_t        albumId = calcId(album + "@" + artist);

    // make sure the record exists so we can update later
    sqlite3_prepare_v2(sqldb, "INSERT OR IGNORE INTO `albums` (`id`) VALUES (?);", -1, &stmt, NULL);
    sqlite3_bind_int64(stmt, 1, albumId);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to insert album: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);

    // don't attempt to update the cover art if the current file doesn't have it
    if (cover.size() != 0) {
        sqlite3_prepare_v2(sqldb,
                        "UPDATE `albums` SET `title`=?, `artistid`=?, `artist`=?, `cover`=?"
                        "WHERE id=?;", -1, &stmt, NULL);
    } else {
        sqlite3_prepare_v2(sqldb,
                        "UPDATE `albums` SET `title`=?, `artistid`=?, `artist`=?"
                        "WHERE id=?;", -1, &stmt, NULL);
    }

    sqlite3_bind_text (stmt, 1, album.c_str(), -1, NULL);
    sqlite3_bind_int64(stmt, 2, calcId(artist));
    sqlite3_bind_text (stmt, 3, artist.c_str(), -1, NULL);
    if (cover.size() != 0) {
        sqlite3_bind_blob (stmt, 4, cover.data(), cover.size(), NULL);
        sqlite3_bind_int64(stmt, 5, albumId);
    } else {
        sqlite3_bind_int64(stmt, 4, albumId);
    }

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to update album: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);
}

void insert_song(sqlite3 * sqldb, string filename, string title, string artist, string album,
    string type, string genre, unsigned tn, unsigned year, unsigned discn, unsigned duration, unsigned bitrate) {

    sqlite3_stmt *stmt;
    sqlite3_prepare_v2(sqldb, "INSERT OR REPLACE INTO `songs` "
        "(`id`, `title`, `albumid`, `album`, `artistid`, `artist`, `type`, `genre`, `trackn`, `year`, `discn`, `duration`, `bitRate`, `filename`)"
        " VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?);", -1, &stmt, NULL);

    sqlite3_bind_int64(stmt, 1, calcId(to_string(tn) + "@" + to_string(discn) + "@" + title + "@" + album + "@" + artist));
    sqlite3_bind_text (stmt, 2, title.c_str(), -1, NULL);
    sqlite3_bind_int64(stmt, 3, calcId(album + "@" + artist));
    sqlite3_bind_text (stmt, 4, album.c_str(), -1, NULL);
    sqlite3_bind_int64(stmt, 5, calcId(artist));
    sqlite3_bind_text (stmt, 6, artist.c_str(), -1, NULL);
    sqlite3_bind_text (stmt, 7, type.c_str(), -1, NULL);
    sqlite3_bind_text (stmt, 8, genre.c_str(), -1, NULL);
    sqlite3_bind_int  (stmt, 9, tn);
    sqlite3_bind_int  (stmt,10, year);
    sqlite3_bind_int  (stmt,11, discn);
    sqlite3_bind_int  (stmt,12, duration);
    sqlite3_bind_int  (stmt,13, bitrate);
    sqlite3_bind_text (stmt,14, filename.c_str(), -1, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to insert song: " << sqlite3_errmsg(sqldb) << " ("<< filename << ")" << endl;
    }

    sqlite3_finalize(stmt);
}

bool scan_music_file(sqlite3 * sqldb, string fullpath) {
    string  artist;
    string  album;
    string  title;
    string  ext = getFileExtension(fullpath);

    std::transform(ext.begin(), ext.end(), ext.begin(), ::tolower);
    if (ext != "mp3" && ext != "ogg" && ext != "flac") {
        return false;
    }

    TagLib::FileRef f(fullpath.c_str());
    if (f.isNull())
        return false;

    TagLib::Tag *tag = f.tag();
    if (!tag)
        return false;


    string cover;
    if (ext == "mp3") {
        TagLib::MPEG::File audioFile(fullpath.c_str());
        TagLib::ID3v2::Tag *mp3_tag = audioFile.ID3v2Tag(true);

        if (mp3_tag) {
            auto frames = mp3_tag->frameList("APIC");
            if (!frames.isEmpty()) {
                auto frame = static_cast<TagLib::ID3v2::AttachedPictureFrame *>(frames.front());
                cover = string(frame->picture().data(), frame->picture().size());
                //frame->mimeType()
            }
        }
    }
    else if (ext == "ogg") {
        auto vorbis_tag = dynamic_cast<TagLib::Ogg::XiphComment *>(tag);
        if (vorbis_tag) {
            if (vorbis_tag->properties().contains("METADATA_BLOCK_PICTURE")) {
                auto cdata = vorbis_tag->properties()["METADATA_BLOCK_PICTURE"][0].data(TagLib::String::UTF8);
                cover = base64Decode(string(cdata.data(), cdata.size()));
                TagLib::FLAC::Picture picture;
                picture.parse(TagLib::ByteVector(cover.c_str(), cover.size()));
                cover = string(picture.data().data(), picture.data().size());
            }
        }
    } else if (ext == "flac") {
        auto flac_tag   = dynamic_cast< TagLib::FLAC::File *>(tag);
        if (flac_tag) {
            TagLib::List<TagLib::FLAC::Picture*> picList = flac_tag->pictureList();
            if (picList.size() > 0) {
                cover = string(picList[0]->data().data(), picList[0]->data().size());
            }
        }
    }

    artist  = trim(tag->artist().toCString(true));
    album   = trim(tag->album().toCString(true));
    title   = trim(tag->title().toCString(true));

    TagLib::AudioProperties *properties = f.audioProperties();
    if (!properties) {
        cout << "ignored " << fullpath << ": no audio metadata present" << endl;
        return false;
    }

    int discn = 0;
    if (tag->properties().contains("DISCNUMBER")) {
        discn = tag->properties()["DISCNUMBER"][0].toInt();
    }

    insert_song(sqldb,
                    fullpath,
                    title,
                    artist,
                    album,
                    ext,
                    trim(tag->genre().toCString(true)),
                    tag->track(),
                    tag->year(),
                    discn,
                    properties->length(),
                    properties->bitrate());

    insert_album(sqldb, album, artist, cover);

    insert_artist(sqldb, artist);

    cout << "added " << fullpath << endl;

    return true;
}

int scan_fs(sqlite3 * sqldb, string name) {
    struct dirent   *entry;
    DIR             *dir;
    int             added_songs = 0;

    if (!(dir = opendir(name.c_str()))) {
         return 0;
    }

    if (!(entry = readdir(dir))) {
         return 0;
    }

    do {
        string fullpath = name + "/" + string(entry->d_name);

        struct stat statbuf;
        stat(fullpath.c_str(), &statbuf);
        if (S_ISDIR(statbuf.st_mode)) {
            if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
                continue;
            added_songs += scan_fs(sqldb, fullpath);
        }
        else {
            if (scan_music_file(sqldb, fullpath)) {
                added_songs++;
            }
        }
    } while ((entry = readdir(dir)));
    closedir(dir);

    return added_songs;
}

void cleanup_db(sqlite3 * sqldb) {
    sqlite3_stmt *stmt;

    // remove albums without any song
    sqlite3_prepare_v2(sqldb,
                        "DELETE FROM `albums` WHERE ("
                        "SELECT count(id) FROM `songs` WHERE albumId = albums.id) = 0;",
                        -1, &stmt, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to cleanup albums: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);

    // remove artists without any album
    sqlite3_prepare_v2(sqldb,
                        "DELETE FROM `artists` WHERE ("
                        "SELECT count(id) FROM `albums` WHERE artistId = artists.id) = 0;",
                        -1, &stmt, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to cleanup artists: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);
}

void truncate_tables(sqlite3 * sqldb) {
    sqlite3_stmt *stmt;

    // truncate songs table
    sqlite3_prepare_v2(sqldb, "DELETE FROM `songs`;", -1, &stmt, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to truncate songs table: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);

    // truncate albums table
    sqlite3_prepare_v2(sqldb, "DELETE FROM `albums`;", -1, &stmt, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to truncate albums table: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);

    // truncate artists table
    sqlite3_prepare_v2(sqldb, "DELETE FROM `artists`;", -1, &stmt, NULL);

    if (sqlite3_step(stmt) != SQLITE_DONE) {
        cerr << "failed to truncate artists table: " << sqlite3_errmsg(sqldb) << endl;
    }
    sqlite3_finalize(stmt);

}

void usage(char* argv[]) {
    fprintf(stderr,
        "Usage: %s action [args...]\n"
        "  %s scan file.db musicdir                 : scan musicdir for new songs\n"
        "  %s fullscan file.db musicdir             : delete all records and do a full rescan\n"
        "  %s useradd file.db username password     : add user\n"
        "  %s userdel file.db username              : delete user\n",
        argv[0],argv[0],argv[0],argv[0], argv[0]);
}

int main(int argc, char* argv[]) {
    sqlite3*    sqldb;
    int         ok;

    if (argc < 3) {
        usage(argv);
        return 1;
    }

    string action = argv[1];
    string dbpath = argv[2];

    // Create a new sqlite db if file does not exist
    ok = sqlite3_open_v2(
        dbpath.c_str(),
        &sqldb,
        SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE,
        NULL
    );
    panic_if(ok != SQLITE_OK, "Could not open sqlite3 database");

    // wait up to 5 secs when the database is being read or written to
    // by another process (allows the server and the indexer to use
    // the db concurrently)
    ok = sqlite3_busy_timeout(sqldb, 5000);
    panic_if(ok != SQLITE_OK, "Could not enable database sharing");

    // make sure the schema exists
    sqlite3_exec(sqldb, init_sql, NULL, NULL, NULL);

    if (action == "scan") {
        string  musicdir    = argv[3];
        int     added       = 0;
        cout << "scanning " << musicdir << "..." << endl;

        // Start scanning and adding stuff to the database
        added   = scan_fs(sqldb, musicdir);
        cout << "added " << added << " files" << endl;

        // cleanup orphaned items
        cout << "cleaning up database..." << endl;
        cleanup_db(sqldb);

    } else if (action == "fullscan") {
        string musicdir     = argv[3];
        int    added        = 0;

        // delete all artists, albums and songs
        cout << "flushing database..." << endl;
        truncate_tables(sqldb);

        // do a full rescan
        cout << "scanning " << musicdir << "..." << endl;
        added   = scan_fs(sqldb, musicdir);
        cout << "added " << added << " files" << endl;

    } else if (action == "useradd") {
        string user = argv[3];
        string pass = argv[4];

        sqlite3_stmt *stmt;
        sqlite3_prepare_v2(sqldb, "INSERT INTO `users` (`username`, `password`) VALUES (?,?);", -1, &stmt, NULL);

        sqlite3_bind_text (stmt, 1, user.c_str(), -1, NULL);
        sqlite3_bind_text (stmt, 2, pass.c_str(), -1, NULL);

        if (sqlite3_step(stmt) != SQLITE_DONE)
            cerr << "failed to add user: " << sqlite3_errmsg(sqldb) << endl;
        sqlite3_finalize(stmt);

    } else if (action == "userdel") {
        string user = argv[3];

        sqlite3_stmt *stmt;
        sqlite3_prepare_v2(sqldb, "DELETE FROM `users` WHERE `username` = ?;", -1, &stmt, NULL);

        sqlite3_bind_text (stmt, 1, user.c_str(), -1, NULL);

        if (sqlite3_step(stmt) != SQLITE_DONE)
            cerr << "failed to remove user: " << sqlite3_errmsg(sqldb) << endl;
        sqlite3_finalize(stmt);

    } else {
        usage(argv);
        return 1;
    }

    // Close and write to disk
    sqlite3_close(sqldb);

    return 0;
}
