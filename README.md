ubersonic
=========
Description
-----------
Lightweight Subsonic server written in go (server) and c++ (media indexer) using sqlite as backend and taglib for indexing.

Dependencies
------------
You'll need libsqlite3 and libtag.

On debian-ish systems, the following should do:
```bash
$ sudo apt-get install libsqlite3 libsqlite3-dev libtag1v5 libtag1-dev
```

On osx, brew has all you need:
```bash
$ brew install sqlite3 taglib
```

Install
-------

Simply running
```bash
$ make all
```
will run a bunch of targets and do its magic in a pretty standard unix way:

* `godeps` will fetch and build github.com/mattn/go-sqlite3
* `build-server` will build the server (golang)
* `build-indexer` will build the indexer (c++)
* `install` will install both executables in /opt/ubersonic/bin/

*make install* will probably fail with a permission denied, in which case simply run
```bash
$ sudo make install
```
Edit *Makefile* to change the install dir.

Run it
------
* first add a user (this will also create the ./ubersonic.db sqlite database)
```bash
$ /opt/ubersonic/bin/ubersonic-indexer adduser ./ubersonic.db headbanger rock$t4r
```

* index some music
```bash
$ /opt/ubersonic/bin/ubersonic-indexer scan ./ubersonic.db /media/terabytes/of/music
```
This will take some time... go for a run or start the server right away.
The server will happily serve content while the indexer is running, serving files as they are indexed.
I run *ubersonic-scanner* from a cron job once a day, grabbing new music automatically while serving music at the same time.

* run the server with TLS support on port 4041
```bash
$ /opt/ubersonic/bin/ubersonic-server --db ./ubersonic.db --cert your-host.com.crt --key your-host.com.key --port 4041
```

You may also run the server without TLS with --notls instead of --cert and --key.
```bash
$ /opt/ubersonic/bin/ubersonic-server --db ./ubersonic.db --notls --port 4041
```
Note that your login/password and media traffic will be sent in the clear.
If you plan on using ubersonic over the internet, get a free tls cert from letsencrypt.

* revoke access for a user
```bash
$ /opt/ubersonic/bin/ubersonic-indexer userdel ./ubersonic.db lameuser
```

* flush the database and re-index your library
```bash
$ /opt/ubersonic/bin/ubersonic-indexer fullscan ./ubersonic.db /media/tons/of/music
```

Implemented API endpoints
-------------------------
* ping.view
* getIndexes.view
* getArtists.view
* getArtist.view
* getAlbum.view
* getSong.view
* stream.view
* download.view

Both XML and JSON encodings are supported (although JSON hasn't really been tested due to lack of iOS clients).

Clients
-------
I've only been using SuperSonic on iOS so far. Works surprisingly well.

Credits
-------
* The original indexer.cpp file comes from https://github.com/davidgfnet/supersonic-cpp/, credits to David G.F.
* Part of the subsonic object definitions come from https://github.com/jeena/sonicmonkey/, credits to Jeena Paradies.
* Owen Worley for building the iOS SuperSonic client.

License
-------
MIT.
