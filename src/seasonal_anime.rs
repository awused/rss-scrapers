use std::collections::{HashMap, HashSet};
use std::io::BufReader;

use anyhow::Result;
use chrono::{NaiveDate, NaiveDateTime, NaiveTime, Utc};
use regex::Regex;
use reqwest::Url;
use rss::{Channel, ChannelBuilder};
use serde::Deserialize;

#[derive(Debug, Deserialize)]
struct Config {
    title: String,
    ongoing: Vec<String>,

    #[serde(flatten)]
    quarters: HashMap<String, Vec<String>>,
}

pub fn get() -> Result<()> {
    let client = reqwest::blocking::Client::new();
    let conf: Config = awconf::load_config("seasonal-anime-rss", None::<&str>, None::<&str>)?.0;

    let quarter_re = Regex::new(r#"^(\d{4})[Qq]([1-4])$"#).unwrap();

    let now = Utc::now();

    let mut searches: HashSet<_> = conf
        .quarters
        .into_iter()
        .filter_map(|(k, v)| {
            let q = quarter_re.captures(&k)?;

            let mut year: i32 = q[1].parse().unwrap();
            let mut quarter: u32 = q[2].parse().unwrap();

            if quarter == 4 {
                year += 1;
                quarter = 0;
            }

            let eoq = NaiveDateTime::new(
                NaiveDate::from_ymd_opt(year, quarter * 3 + 1, 14).unwrap(),
                NaiveTime::default(),
            )
            .and_utc();
            if eoq > now { Some(v) } else { None }
        })
        .flatten()
        .collect();

    conf.ongoing.into_iter().for_each(|s| {
        searches.insert(s);
    });

    let search = "(".to_string()
        + &searches.iter().map(String::as_str).collect::<Vec<_>>().join(")|(")
        + ")";

    let mut rss_url = Url::parse("https://nyaa.si/?page=rss").unwrap();
    let mut search_url = Url::parse("https://nyaa.si/").unwrap();

    rss_url.query_pairs_mut().append_pair("q", &search);
    search_url.query_pairs_mut().append_pair("q", &search);

    let mut feed = ChannelBuilder::default();
    feed.title(&conf.title).link(search_url.to_string()).ttl(Some("60".into()));

    if searches.is_empty() {
        print!("{}", feed.build().to_string());
        return Ok(());
    }

    let base_feed = client.get(rss_url).send()?;

    // If we need feed-rs for another scraper, use that instead for more generalized parsing?
    let base_feed = Channel::read_from(BufReader::new(base_feed))?;
    feed.items(base_feed.items);

    print!("{}", feed.build().to_string());
    Ok(())
}
