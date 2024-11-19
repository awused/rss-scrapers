use chrono::Utc;
use color_eyre::Result;
use rss::{ChannelBuilder, GuidBuilder, ItemBuilder};
use scraper::{Html, Selector};
use tracing::error_span;

const URL: &str = "https://erogegames.com/forums/forum/14-eroge-news/";

pub fn get() -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let html = client.get(URL).send()?.bytes()?;
    let _span = error_span!("response", html = %String::from_utf8_lossy(&html)).entered();

    let html = String::from_utf8(html.into())?;
    let doc = Html::parse_document(&html);

    // Page does not have times, but using the current time is good enough
    let now = Utc::now().to_rfc2822();

    let items: Vec<_> = doc
        .select(&Selector::parse("a[title*=\"Visual Novel Translation\"]").unwrap())
        .filter(|a| !a.attr("title").unwrap().contains("H-RPG"))
        .map(|a| {
            let href = a.attr("href").unwrap();

            ItemBuilder::default()
                .title(a.attr("title").map(str::to_string))
                .link(Some(href.into()))
                .guid(Some(GuidBuilder::default().value(href.to_string()).build()))
                .pub_date(Some(now.clone()))
                .build()
        })
        .collect();


    let feed = ChannelBuilder::default()
        .title("Visual Novel Translation Status".to_string())
        .link(URL.to_string())
        .items(items)
        .ttl(Some(360.to_string()))
        .build();

    print!("{}", feed.to_string());
    Ok(())
}
