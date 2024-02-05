use std::collections::{HashMap, HashSet};
use std::thread;
use std::time::Duration;

use anyhow::{bail, Result};
use chrono::DateTime;
use reqwest::blocking::Client;
use reqwest::Url;
use rss::{ChannelBuilder, GuidBuilder, Item, ItemBuilder};
use serde::Deserialize;
use serde_with::{serde_as, DefaultOnNull, NoneAsEmptyString};

const DELAY: Duration = Duration::from_secs(2);

static USER_AGENT: &str = concat!(env!("CARGO_PKG_NAME"), "/", env!("CARGO_PKG_VERSION"),);

const PAGE_SIZE: usize = 100;


#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "PascalCase")]
struct MangaSyncerConfig {
    // Use the BlockedGroups field from manga-syncer
    #[serde(default)]
    blocked_groups: Vec<String>,
}

pub fn get(series: String) -> Result<()> {
    let client = Client::builder().user_agent(USER_AGENT).build()?;

    thread::sleep(DELAY);

    let info: MangaInfo =
        client.get(format!("https://api.mangadex.org/manga/{series}")).send()?.json()?;

    if info.result != "ok" {
        bail!("Failed to get info for {series}: {info:?}");
    }

    let title = english_or_first(&info.data.attributes.title).unwrap_or_default();
    let description = english_or_first(&info.data.attributes.description).unwrap_or_default();


    let feed = ChannelBuilder::default()
        .description(description)
        .link(format!("https://mangadex.org/title/{series}"))
        .ttl(Some("60".into()))
        .items(get_chapters(&client, &series, &title)?)
        .title(title)
        .build();

    println!("{}", feed.to_string());
    Ok(())
}


fn get_chapters(client: &Client, series: &str, title: &str) -> Result<Vec<Item>> {
    let manga_syncer_config: MangaSyncerConfig =
        awconf::load_config("manga-syncer", None::<&str>, Some(""))?.0;
    let blocked_groups: HashSet<_> = manga_syncer_config.blocked_groups.into_iter().collect();

    let mut total = 1;
    let mut offset = 0;

    let mut page_url = Url::parse(&format!("https://api.mangadex.org/manga/{series}/feed"))?;

    page_url
        .query_pairs_mut()
        .append_pair("limit", &PAGE_SIZE.to_string())
        .append_pair("translatedLanguage[]", "en")
        .append_pair("order[chapter]", "desc");

    let mut chapters = Vec::new();

    while offset < total {
        thread::sleep(DELAY);

        let mut url = page_url.clone();
        url.query_pairs_mut().append_pair("offset", &offset.to_string());

        let page: ChapterList = client.get(url).send()?.json()?;

        total = page.total as usize;
        if page.data.len() != PAGE_SIZE && offset + page.data.len() < total {
            bail!(
                "Manga {series}: invalid chapter pagination. Requested {PAGE_SIZE} chapters at \
                 offset {offset} with {total} total but got {}",
                page.data.len()
            );
        }

        chapters.extend(
            page.data
                .into_iter()
                .filter(|c| {
                    !c.relationships.iter().any(|r| {
                        r.type_field == "scanlation_group" && blocked_groups.contains(&r.id)
                    })
                })
                .map(|c| {
                    let title =
                        match (c.attributes.volume, c.attributes.chapter, c.attributes.title) {
                            (Some(v), Some(c), Some(t)) => {
                                format!("{title} - Volume {v}, Chapter {c} - {t}")
                            }
                            (Some(v), Some(c), None) => {
                                format!("{title} - Volume {v}, Chapter {c}")
                            }
                            (None, Some(c), Some(t)) => {
                                format!("{title} - Chapter {c} - {t}")
                            }
                            (None, None, Some(t)) => {
                                format!("{title} - {t}")
                            }
                            (..) => format!("{title} -- unknown chapter"),
                        };

                    // This is probably unnecessary (aw-rss will consume rfc3339) but matches the
                    // old Go code exactly
                    let pub_date = DateTime::parse_from_rfc3339(&c.attributes.created_at)
                        .unwrap()
                        .to_utc()
                        .to_rfc2822();

                    ItemBuilder::default()
                        .title(Some(title))
                        .link(Some(format!("https://mangadex.org/chapter/{}", c.id)))
                        .guid(Some(GuidBuilder::default().value(c.id).build()))
                        .pub_date(Some(pub_date))
                        .build()
                }),
        );


        offset += PAGE_SIZE;
    }

    Ok(chapters)
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MangaInfo {
    pub result: String,
    pub data: Data,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Data {
    pub attributes: MangaAttributes,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MangaAttributes {
    pub title: LocalizedString,
    pub description: LocalizedString,
}

type LocalizedString = HashMap<String, String>;

fn english_or_first(s: &LocalizedString) -> Option<String> {
    s.get("en").or_else(|| s.values().next()).map(String::clone)
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Relationship {
    pub id: String,
    #[serde(rename = "type")]
    pub type_field: String,
}


#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ChapterList {
    pub data: Vec<Chapter>,
    pub total: i64,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Chapter {
    pub id: String,
    pub attributes: ChapterAttributes,
    pub relationships: Vec<Relationship>,
}

#[serde_as]
#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ChapterAttributes {
    #[serde_as(as = "NoneAsEmptyString")]
    pub volume: Option<String>,
    #[serde_as(deserialize_as = "DefaultOnNull")]
    pub chapter: Option<String>,
    #[serde_as(as = "NoneAsEmptyString")]
    pub title: Option<String>,
    pub created_at: String,
}
