use std::collections::HashSet;

use anyhow::Result;
use chrono::Utc;
use reqwest::blocking::Client;
use rss::{ChannelBuilder, GuidBuilder, Item, ItemBuilder};
use serde::Deserialize;

pub fn get(series: String) -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let info: SeriesInfo = client
        .get(format!(r#"https://api.j-novel.club/api/series/findOne?filter={{"where":{{"titleslug":"{series}"}},"include":["volumes","parts"]}}"#))
        .send()?
        .json()?;


    let finals = final_chapters(&client, &info.id)?;

    let now = Utc::now().to_rfc2822();

    let items: Vec<Item> = info
        .parts
        .into_iter()
        .rev()
        .filter(|p| !p.expired)
        .map(|p| {
            let fragment = format!("/c/{}", p.titleslug);
            let mut title = p.title;
            if finals.contains(&fragment) {
                title += " FINAL";
            }

            ItemBuilder::default()
                .title(Some(title))
                .link(Some(format!("https://j-novel.club{fragment}")))
                .guid(Some(GuidBuilder::default().value(p.titleslug).build()))
                .pub_date(Some(now.clone()))
                .build()
        })
        .collect();


    let feed = ChannelBuilder::default()
        .title(info.title)
        .link(format!("https://j-novel.club/series/{series}"))
        .description(info.description_short)
        .ttl(Some("60".into()))
        .items(items)
        .build();

    print!("{}", feed.to_string());
    Ok(())
}

fn final_chapters(client: &Client, id: &str) -> Result<HashSet<String>> {
    let events: Vec<Event> = client
        .get(format!(
            "https://api.j-novel.club/api/events?filter[limit]=100&filter[where][serieId]={id}"
        ))
        .send()?
        .json()?;

    Ok(events
        .into_iter()
        .filter(|e| e.details.ends_with("FINAL"))
        .map(|e| e.link_fragment)
        .collect())
}


#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct SeriesInfo {
    pub title: String,
    pub description_short: String,
    pub id: String,
    pub parts: Vec<Part>,
}

#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Part {
    pub title: String,
    pub titleslug: String,
    pub expired: bool,
}


#[derive(Default, Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Event {
    pub details: String,
    pub link_fragment: String,
}
