use chrono::Utc;
use color_eyre::Result;
use color_eyre::eyre::OptionExt;
use rss::{ChannelBuilder, GuidBuilder, ItemBuilder};
use scraper::{Html, Selector};
use tracing::error_span;

const HOST: &str = "https://archiveofourown.org";

pub fn get(series: String) -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let navigate = format!("{HOST}/works/{series}/navigate");

    let html = client.get(&navigate).send()?.bytes()?;

    let _span = error_span!("document", document = %String::from_utf8_lossy(&html)).entered();

    let html = String::from_utf8(html.into())?;
    let doc = Html::parse_document(&html);

    // Page does not have times, but using the current time is good enough
    let now = Utc::now().to_rfc2822();


    let title = doc
        .select(&Selector::parse("h2.heading > a").unwrap())
        .next()
        .ok_or_eyre("No title")?
        .text()
        .next()
        .ok_or_eyre("Title had no text")?;

    let chapters: Vec<_> = doc
        .select(&Selector::parse("ol.chapter > li > a").unwrap())
        .rev()
        .map(|c| {
            let title = c.text().next().ok_or_eyre("Chapter has no title").unwrap();
            let href = c.attr("href").ok_or_eyre("Missing chapter link").unwrap();

            ItemBuilder::default()
                .title(Some(title.to_string()))
                .link(Some(format!("{HOST}{href}")))
                .guid(Some(GuidBuilder::default().value(href.to_string()).build()))
                .pub_date(Some(now.clone()))
                .build()
        })
        .collect();

    let feed = ChannelBuilder::default()
        .title(title.to_string())
        .link(navigate)
        .ttl(Some("360".into()))
        .items(chapters)
        .build();

    print!("{feed}");
    Ok(())
}
