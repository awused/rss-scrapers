use anyhow::{Context, Result};
use chrono::Utc;
use rss::{ChannelBuilder, GuidBuilder, ItemBuilder};
use scraper::{CaseSensitivity, Html, Selector};

pub fn get(game: String) -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let url = format!("https://tfgames.site/?module=viewgame&id={game}");
    let a_select = Selector::parse(".dltext > a").unwrap();

    let page = client.get(url.clone()).send()?.text()?;
    let doc = Html::parse_document(&page);
    let title = doc
        .select(&Selector::parse("title").unwrap())
        .next()
        .context("No title")?
        .text()
        .collect::<String>();

    let now = Utc::now().to_rfc2822();

    let mut version = String::new();
    let mut items = Vec::new();

    for e in doc.select(&Selector::parse("div#downloads > *").unwrap()) {
        if e.value().name() == "center" {
            version = e.text().collect();
            continue;
        }

        if !e.value().has_class("dlcontainer", CaseSensitivity::CaseSensitive) {
            continue;
        }

        let a = e.select(&a_select).next().context("Missing download link")?;

        let href = a.attr("href").context("Download link missing url")?;

        items.push(
            ItemBuilder::default()
                .title(Some(format!("{version} {}", a.text().collect::<String>())))
                .link(Some(href.to_string()))
                .guid(Some(GuidBuilder::default().value(href.to_string()).build()))
                .pub_date(Some(now.clone()))
                .build(),
        );
    }

    let feed = ChannelBuilder::default()
        .title(title)
        .link(url)
        .ttl(Some(360.to_string()))
        .items(items)
        .build();

    println!("{}", feed.to_string());
    Ok(())
}
