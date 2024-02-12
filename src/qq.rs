use std::fs::File;
use std::io::{BufReader, ErrorKind};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use anyhow::{Context, Result};
use chrono::{TimeZone, Utc};
use reqwest::blocking::multipart::Form;
use reqwest_cookie_store::{CookieStore, CookieStoreMutex};
use rss::{ChannelBuilder, GuidBuilder, ItemBuilder};
use scraper::{Html, Selector};
use serde::Deserialize;


#[derive(Debug, Deserialize)]
struct Config {
    username: String,
    password: String,
    cookie_jar: PathBuf,
}

pub fn get(thread_id: String) -> Result<()> {
    let config: Config = awconf::load_config("qq-rss", None::<&str>, None::<&str>)?.0;

    let cookie_store = match File::open(&config.cookie_jar) {
        Ok(f) => CookieStore::load_json(BufReader::new(f)).unwrap(),
        Err(e) if e.kind() == ErrorKind::NotFound => CookieStore::new(None),
        Err(e) => return Err(e.into()),
    };

    let cookie_store = Arc::new(CookieStoreMutex::new(cookie_store));

    let client = reqwest::blocking::Client::builder()
        .cookie_provider(cookie_store.clone())
        .build()?;

    let url = format!(
        "https://forum.questionablequesting.com/threads/{thread_id}/threadmarks?category_id=1"
    );
    let text = client.get(&url).send()?.text()?;

    let doc = Html::parse_document(&text);
    let threadmarks = Selector::parse(".threadmarkList").unwrap();


    let doc = if doc.select(&threadmarks).next().is_none() {
        thread::sleep(Duration::from_secs(1));

        let form = Form::new()
            .text("login", config.username)
            .text("register", "0")
            .text("password", config.password)
            .text("remember", "1")
            .text("cookie_check", "1")
            .text("_xfToken", "");

        client
            .post("https://forum.questionablequesting.com/login/login")
            .multipart(form)
            .send()?
            .text()?;


        thread::sleep(Duration::from_secs(1));
        let text = client.get(&url).send()?.text()?;

        Html::parse_document(&text)
    } else {
        doc
    };

    {
        let mut f = File::create(&config.cookie_jar)?;
        let store = cookie_store.lock().unwrap();
        store.save_json(&mut f).unwrap();
    }

    let mut items = Vec::new();
    let link = Selector::parse("a.PreviewTooltip").unwrap();
    for mark in doc.select(&Selector::parse(".threadmarkListItem").unwrap()).rev() {
        let link = mark.select(&link).next().context("Link missing from threadmark")?;
        let href = link.attr("href").context("Link missing href")?;
        let title = link.text().collect::<String>().trim().to_string();
        let url = format!("https://forum.questionablequesting.com/{href}");
        let created: i64 = mark.attr("data-content-date").context("Missing date")?.parse()?;
        let created = Utc.timestamp_opt(created, 0).unwrap().to_rfc2822();

        let guid = href.split_once("post-").map_or(href, |p| p.1);


        items.push(
            ItemBuilder::default()
                .title(Some(title))
                .link(Some(url))
                .guid(Some(GuidBuilder::default().value(guid.to_string()).build()))
                .pub_date(Some(created))
                .build(),
        )
    }

    let thread_link = doc
        .select(&Selector::parse(".crust:last-child > .crumb").unwrap())
        .next()
        .context("No link to thread.")?;
    let title = thread_link.text().collect::<String>();
    let title = title.strip_prefix("[NSFW]").map(str::trim).map(str::to_string).unwrap_or(title);

    let feed = ChannelBuilder::default()
        .title(title)
        .link(thread_link.attr("href").context("Thread link missing href")?.to_string())
        .items(items)
        .ttl(Some(120.to_string()))
        .build();

    print!("{}", feed.to_string());
    Ok(())
}
