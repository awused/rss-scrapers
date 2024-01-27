#! /usr/bin/env python

import sqlite3
import sys

mangaMap = dict()
chapterMap = dict()

con = sqlite3.connect(sys.argv[1])

cur = con.cursor()

cur.execute(
    'SELECT key FROM items WHERE url LIKE "https://mangadex.org/chapter/%"')

chapterUpdates = []
for r in cur.fetchall():
    cid = str(r[0]).removeprefix('https://mangadex.org/chapter/')
    chapterUpdates.append((cid, r[0]))

# print(chapterUpdates[:20])
cur.executemany('UPDATE items SET key = ? WHERE url = ?', chapterUpdates)

cur.execute('SELECT url FROM feeds WHERE url LIKE "!mangadex-rss %"')

feedUpdates = []
for r in cur.fetchall():
    mid = str(r[0]).rsplit(' ', 1)[1]
    feedUpdates.append(('!rss-scrapers mangadex ' + mid, r[0]))

# print(feedUpdates[:20])
cur.executemany('UPDATE feeds SET url = ? WHERE url = ?', feedUpdates)

cur.close()
con.commit()
con.close()
