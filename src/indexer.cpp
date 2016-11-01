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

void insert_artist(sqlite3 * sqldb, string artist) {
    sqlite3_stmt *stmt;
    sqlite3_prepare_v2(sqldb, "INSERT OR REPLACE INTO `artists` (`id`, `name`) VALUES (?,?);", -1, &stmt, NULL);

    sqlite3_bind_int64(stmt, 1, calcId(artist));
    sqlite3_bind_text (stmt, 2, artist.c_str(), -1, NULL);

    sqlite3_step(stmt);
    sqlite3_finalize(stmt);
}

void insert_album(sqlite3 * sqldb, string album, string artist, string cover) {
    sqlite3_stmt *stmt;
    sqlite3_prepare_v2(sqldb, "INSERT OR REPLACE INTO `albums` "
        "(`id`, `title`, `artistid`, `artist`, `cover`) VALUES (?,?,?,?,?);", -1, &stmt, NULL);

    sqlite3_bind_int64(stmt, 1, calcId(album + "@" + artist));
    sqlite3_bind_text (stmt, 2, album.c_str(), -1, NULL);
    sqlite3_bind_int64(stmt, 3, calcId(artist));
    sqlite3_bind_text (stmt, 4, artist.c_str(), -1, NULL);
    sqlite3_bind_blob (stmt, 5, cover.data(), cover.size(), NULL);

    sqlite3_step(stmt);
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
        cout << "Err " << filename << endl;
        cout << sqlite3_errmsg(sqldb) << endl;
    }

    sqlite3_finalize(stmt);
}

void scan_music_file(sqlite3 * sqldb, string fullpath) {
    string ext = fullpath.substr(fullpath.size()-3);
    std::transform(ext.begin(), ext.end(), ext.begin(), ::tolower);

    TagLib::FileRef f(fullpath.c_str());
    if (f.isNull())
        return;

    TagLib::Tag *tag = f.tag();
    if (!tag)
        return;


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
    }

    if (ext == "mp3" or ext == "ogg") {
        TagLib::AudioProperties *properties = f.audioProperties();
        if (!properties)
            return;

        int discn = 0;
        if (tag->properties().contains("DISCNUMBER")) {
            discn = tag->properties()["DISCNUMBER"][0].toInt();
        }

        insert_song(sqldb, fullpath, tag->title().toCString(true), tag->artist().toCString(true),
            tag->album().toCString(true), ext, tag->genre().toCString(true),
            tag->track(), tag->year(), discn, properties->length(), properties->bitrate());

        insert_album(sqldb, tag->album().toCString(true), tag->artist().toCString(true), cover);
        insert_artist(sqldb, tag->artist().toCString(true));

	cout << "added " << fullpath << "\n";
    }
}

void scan_fs(sqlite3 * sqldb, string name) {
    DIR *dir;
    struct dirent *entry;

    if (!(dir = opendir(name.c_str())))  return;
    if (!(entry = readdir(dir))) return;

    do {
        string fullpath = name + "/" + string(entry->d_name);
        string ext = fullpath.substr(fullpath.size()-3);

        struct stat statbuf;
        stat(fullpath.c_str(), &statbuf);
        if (S_ISDIR(statbuf.st_mode)) {
            if (strcmp(entry->d_name, ".") == 0 || strcmp(entry->d_name, "..") == 0)
                continue;
            scan_fs(sqldb, fullpath);
        }
        else
            scan_music_file(sqldb, fullpath);
    } while (entry = readdir(dir));
    closedir(dir);
}


int main(int argc, char* argv[]) {
    if (argc < 3) {
        fprintf(stderr,
            "Usage: %s action [args...]\n"
            "  %s scan file.db musicdir/ \n"
            "  %s useradd file.db username password\n"
            "  %s userdel file.db username\n",
            argv[0],argv[0],argv[0],argv[0]);
        return 1;
    }
    string action = argv[1];
    string dbpath = argv[2];

    // Create a new sqlite db if file does not exist
    sqlite3 * sqldb;
    int ok = sqlite3_open_v2(
        dbpath.c_str(),
        &sqldb,
        SQLITE_OPEN_READWRITE | SQLITE_OPEN_CREATE,
        NULL
    );
    panic_if(ok != SQLITE_OK, "Could not open sqlite3 database!");

    if (action == "scan") {
        string musicdir = argv[3];

        sqlite3_exec(sqldb, init_sql, NULL, NULL, NULL);

	cout << "scanning " << musicdir << "\n";
        // Start scanning and adding stuff to the database
        scan_fs(sqldb, musicdir);
    }
    if (action == "useradd") {
        string user = argv[3];
        string pass = argv[4];

        sqlite3_stmt *stmt;
        sqlite3_prepare_v2(sqldb, "INSERT INTO `users` (`username`, `password`) VALUES (?,?);", -1, &stmt, NULL);

        sqlite3_bind_text (stmt, 1, user.c_str(), -1, NULL);
        sqlite3_bind_text (stmt, 2, pass.c_str(), -1, NULL);

        if (sqlite3_step(stmt) != SQLITE_DONE)
            cerr << "Error adding user " << sqlite3_errmsg(sqldb) << endl;
        sqlite3_finalize(stmt);
    }

    // Close and write to disk
    sqlite3_close(sqldb);

    return 0;
}
