#! /usr/bin/env python

import sqlite3
import sys

mangaMap = dict()
chapterMap = dict()

con = sqlite3.connect(sys.argv[1])

cur = con.cursor()

cur.execute('SELECT url FROM feeds WHERE url LIKE "!gelbooru-rss %"')

feedUpdates = []
for r in cur.fetchall():
    rest = str(r[0]).rsplit(' ', 1)[1]
    feedUpdates.append(('!rss-scrapers gelbooru ' + rest, r[0]))

# print(feedUpdates[:20])
cur.executemany('UPDATE feeds SET url = ? WHERE url = ?', feedUpdates)

cur.close()
con.commit()
con.close()
