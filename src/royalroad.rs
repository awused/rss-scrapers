use chrono::Utc;
use color_eyre::Result;
use regex::Regex;
use reqwest::blocking::Client;
use rss::{ChannelBuilder, GuidBuilder, Item, ItemBuilder};
use scraper::{Html, Selector};
use tracing::error_span;

pub fn get() -> Result<()> {
    let client = reqwest::blocking::Client::new();

    let mut items =
        get_fictions(&client, "https://www.royalroad.com/fictions/trending".to_string())?;

    items.extend(get_fictions(
        &client,
        "https://www.royalroad.com/fictions/rising-stars".to_string(),
    )?);

    for page in 1..=5 {
        items.extend(get_fictions(
            &client,
            format!("https://www.royalroad.com/fictions/weekly-popular?page={page}"),
        )?);
    }

    let feed = ChannelBuilder::default()
        .title("Royal Road - Trending/Popular".to_string())
        .link("https://www.royalroad.com/fictions/trending".to_string())
        .ttl(Some((60 * 12).to_string()))
        .items(items)
        .build();

    print!("{feed}");
    Ok(())
}

fn get_fictions(client: &Client, url: String) -> Result<Vec<Item>> {
    let selector = Selector::parse("h2.fiction-title > a").unwrap();
    let re = Regex::new(r#"^/fiction/(\d+)(/|$)"#).unwrap();

    let page = client.get(url).send()?.bytes()?;
    let _span = error_span!("response", page = %String::from_utf8_lossy(&page)).entered();

    let page = String::from_utf8(page.into())?;
    let doc = Html::parse_document(&page);

    let now = Utc::now().to_rfc2822();

    Ok(doc
        .select(&selector)
        .map(|a| {
            let title: String = a.text().collect();
            let href = a.attr("href").unwrap();

            let cap = re.captures(href).unwrap();

            ItemBuilder::default()
                .title(Some(title))
                .link(Some(format!("https://www.royalroad.com{href}")))
                .guid(Some(GuidBuilder::default().value(cap[1].to_string()).build()))
                .pub_date(Some(now.clone()))
                .build()
        })
        .collect())
}
