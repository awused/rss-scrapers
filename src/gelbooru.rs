use std::collections::HashSet;
use std::thread;
use std::time::Duration;

use chrono::DateTime;
use color_eyre::Result;
use color_eyre::eyre::{bail, eyre};
use once_cell::sync::Lazy;
use reqwest::Url;
use rocksdb::DB;
use rss::{ChannelBuilder, GuidBuilder, ItemBuilder};
use serde::Deserialize;
use serde_with::{NoneAsEmptyString, serde_as};
use tracing::error_span;

static CONFIG: Lazy<Config> =
    Lazy::new(|| awconf::load_config("gelbooru-rss", None::<&str>, Some("")).unwrap().0);

const DELAY: Duration = Duration::from_secs(1);

#[serde_as]
#[derive(Debug, Deserialize)]
struct Config {
    #[serde(default)]
    #[serde_as(as = "NoneAsEmptyString")]
    user_id: Option<String>,
    #[serde(default)]
    #[serde_as(as = "NoneAsEmptyString")]
    api_key: Option<String>,
    #[serde(default)]
    #[serde_as(as = "NoneAsEmptyString")]
    tag_db: Option<String>,
    #[serde(default)]
    blacklist: HashSet<String>,
}

pub fn get(query: Vec<String>) -> Result<()> {
    let config = Lazy::force(&CONFIG);
    let db = open_db()?;
    let client = reqwest::blocking::Client::new();

    let mut tags = query.iter().map(|q| urlencoding::encode(q)).collect::<Vec<_>>().join("+");

    let mut api_url = Url::parse(&format!(
        "https://gelbooru.com/index.php?page=dapi&s=post&q=index&json=1&tags={tags}"
    ))?;

    add_api_params(&mut api_url);

    let response = client.get(api_url).send()?.bytes()?;

    let _span = error_span!("response", response = %String::from_utf8_lossy(&response)).entered();

    let index: IndexResponse = serde_json::from_slice(&response)?;
    let mut matched_blacklist_tags = HashSet::new();

    let items = index
        .post
        .into_iter()
        .filter(|p| {
            if let Some(b) = p.tags.split(' ').find(|t| config.blacklist.contains(*t)) {
                matched_blacklist_tags.insert(config.blacklist.get(b).unwrap().as_str());
                return false;
            }
            true
        })
        .map(|p| {
            let title = get_title_for_image(&db, &p, &query)?;

            // Mon Dec 05 08:26:31 -0600 2022
            let pub_date = DateTime::parse_from_str(&p.created_at, "%a %b %d %H:%M:%S %z %Y")?
                .to_utc()
                .to_rfc2822();

            Ok(ItemBuilder::default()
                .title(Some(title))
                .guid(Some(GuidBuilder::default().value(p.id.to_string()).build()))
                .link(Some(format!("https://gelbooru.com/index.php?page=post&s=view&id={}", p.id)))
                .pub_date(Some(pub_date))
                .build())
        })
        .collect::<Result<Vec<_>>>()?;

    db.flush()?;

    if !matched_blacklist_tags.is_empty() {
        let mut blacklisted = matched_blacklist_tags.into_iter().collect::<Vec<_>>();
        blacklisted.sort();
        tags += "+-";
        tags += &blacklisted.join("+-");
    }


    let feed = ChannelBuilder::default()
        .title(query.join(", "))
        .link(format!("https://gelbooru.com/index.php?page=post&s=list&tags={tags}"))
        .description(query.join(", ") + " - Gelbooru")
        .ttl(Some(120.to_string()))
        .items(items)
        .build();

    print!("{feed}");
    Ok(())
}

fn open_db() -> Result<DB> {
    let mut opts = rocksdb::Options::default();
    opts.create_if_missing(true);
    opts.create_missing_column_families(true);
    opts.set_compression_type(rocksdb::DBCompressionType::Lz4);
    opts.set_max_open_files(100);
    opts.set_keep_log_file_num(10);

    let path = CONFIG.tag_db.as_ref().map_or_else(
        || {
            let mut p = dirs::home_dir().unwrap();
            p.push(".rss");
            p.push("geltagdb");
            p
        },
        Into::into,
    );

    let Ok(db) = DB::open(&opts, &path) else {
        DB::repair(&opts, &path)?;
        return Ok(DB::open(&opts, &path)?);
    };
    Ok(db)
}

fn add_api_params(url: &mut Url) {
    let mut pairs = url.query_pairs_mut();
    if let Some(user_id) = &CONFIG.user_id {
        pairs.append_pair("user_id", user_id);
    }
    if let Some(api_key) = &CONFIG.api_key {
        pairs.append_pair("api_key", api_key);
    }
}

fn get_title_for_image(db: &DB, post: &Post, query: &[String]) -> Result<String> {
    let mut relevant_tags = HashSet::new();

    let missing_tags: Vec<_> = post
        .tags
        .split(' ')
        .map(|t| (t, db.get(t)))
        .filter_map(|(t, r)| match r {
            Ok(Some(n)) if !n.is_empty() => {
                if tag_in_title(t, n[0], query) {
                    relevant_tags.insert(t);
                }
                None
            }
            Ok(_) => Some(Ok(t)),
            Err(e) => Some(Err(e.into())),
        })
        .collect::<Result<_>>()?;

    // Fetch any missing tags
    missing_tags
        .chunks(50)
        .map(|c| {
            thread::sleep(DELAY);
            load_missing_tags(db, c)
        })
        .collect::<Result<Vec<_>>>()?;

    missing_tags
        .into_iter()
        .map(|t| (t, db.get(t)))
        .filter_map(|(t, r)| match r {
            Ok(Some(n)) if !n.is_empty() => {
                if tag_in_title(t, n[0], query) {
                    relevant_tags.insert(t);
                }
                None
            }
            Ok(_) => Some(Err(eyre!("Unable to read tag_type for {t}"))),
            Err(e) => Some(Err(e.into())),
        })
        .collect::<Result<()>>()?;


    let mut title = if relevant_tags.is_empty() {
        post.id.to_string()
    } else {
        post.tags
            .split(' ')
            .filter(|t| relevant_tags.contains(t))
            .map(|t| html_escape::decode_html_entities(t))
            .collect::<Vec<_>>()
            .join(", ")
    };

    title.push_str(" - ");
    title.push_str(&post.md5);
    Ok(title)
}

fn tag_in_title(tag: &str, tag_type: u8, query: &[String]) -> bool {
    // 0: "tag"
    // 1: "artist"
    // 3: "copyright"
    // 4: "character"
    // 5: "metadata"
    // 6: "deprecated"
    if tag_type != 4 && tag_type != 3 && tag_type != 1 {
        return false;
    }

    !query.iter().any(|t| t == tag)
}

fn load_missing_tags(db: &DB, tags: &[&str]) -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let escaped = tags
        .iter()
        .map(|q| urlencoding::encode(&html_escape::decode_html_entities(&q)).to_string())
        .collect::<Vec<_>>()
        .join("+");

    let mut tags_url = Url::parse(&format!(
        "https://gelbooru.com/index.php?page=dapi&s=tag&q=index&json=1&names={escaped}"
    ))?;
    add_api_params(&mut tags_url);

    let response = client.get(tags_url).send()?.bytes()?;

    let _span = error_span!("load_missing_tags", response = %String::from_utf8_lossy(&response));

    let response: TagsResponse = serde_json::from_slice(&response)?;

    // Some tags are duplicated, how. Why.
    // Response isn't in any particular order either.
    let mut unmatched: HashSet<&str> = tags.iter().copied().collect();
    assert!(unmatched.len() == response.tag.len());


    for tag in &response.tag {
        db.put(&tag.name, vec![tag.type_field.try_into()?])?;

        // Some tags are just different from different APIs. Fun.
        if !unmatched.remove(tag.name.as_str()) {
            let lower = tag.name.to_lowercase();
            if unmatched.remove(lower.as_str()) {
                db.put(&lower, vec![tag.type_field.try_into()?])?
            }
        }
    }

    if !unmatched.is_empty() {
        bail!("Got unmatched tags {unmatched:?} in response {response:?}");
    }

    Ok(())
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct IndexResponse {
    // This will not be present if there are no posts, but best to fail loudly
    pub post: Vec<Post>,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Post {
    pub id: i64,
    #[serde(rename = "created_at")]
    pub created_at: String,
    pub md5: String,
    pub tags: String,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct TagsResponse {
    pub tag: Vec<Tag>,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Tag {
    pub name: String,
    #[serde(rename = "type")]
    pub type_field: i64,
}
