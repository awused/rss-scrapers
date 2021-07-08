#! /usr/bin/env python

import sqlite3
import sys

mangaMap = dict()
chapterMap = dict()

# Args - DB, manga map, chapter map
with open(sys.argv[2], 'r') as f:
    for line in f:
        s = line.strip().split(',')
        mangaMap[s[1]] = s[2]

with open(sys.argv[3], 'r') as f:
    for line in f:
        s = line.strip().split(',')
        chapterMap[s[1]] = s[2]

con = sqlite3.connect(sys.argv[1])

cur = con.cursor()

cur.execute(
    'SELECT url FROM items WHERE url LIKE "https://mangadex.org/chapter/%"')

chapterUpdates = []
for r in cur.fetchall():
    cid = str(r[0]).removeprefix('https://mangadex.org/chapter/')
    if cid not in chapterMap:
        print("Unmatched chapter ID, was likely deleted", cid)
        continue
    url = 'https://mangadex.org/chapter/' + chapterMap[cid]
    chapterUpdates.append((url, url, r[0]))

cur.executemany('UPDATE items SET url = ?, key = ? WHERE url = ?',
                chapterUpdates)

cur.execute(
    'SELECT url FROM feeds WHERE url LIKE "https://mangadex.org/rss/%"')

feedUpdates = []
for r in cur.fetchall():
    mid = str(r[0]).rsplit('/', 1)[1]
    if mid not in mangaMap:
        print('Unmatched manga ID, was likely deleted', mid)
        continue
    feedUpdates.append(('!mangadex-rss ' + mangaMap[mid], r[0]))

cur.executemany('UPDATE feeds SET url = ? WHERE url = ?', feedUpdates)

cur.close()
con.commit()
con.close()
