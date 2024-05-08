use std::fs::File;
use std::io::{BufReader, Cursor, ErrorKind};
use std::path::PathBuf;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use anyhow::{Context, Result};
use reqwest::blocking::multipart::Form;
use reqwest_cookie_store::{CookieStore, CookieStoreMutex};
use rss::Channel;
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
        "https://forum.questionablequesting.com/threads/{thread_id}/threadmarks.rss?category_id=1"
    );
    let text = client.get(&url).send()?.text()?;
    let feed = Channel::read_from(Cursor::new(&text));

    let mut feed = match feed {
        Ok(feed) => feed,
        Err(_e) => {
            let text = client.get("https://forum.questionablequesting.com").send()?.text()?;
            let doc = Html::parse_document(&text);

            // We don't need a specific xsrf token for the login page, any valid token will do.
            let xf_token = doc
                .select(&Selector::parse("input[name=\"_xfToken\"]").unwrap())
                .next()
                .context("No xfToken in initial response")?
                .attr("value")
                .context("xfToken had no value")?
                .to_string();

            thread::sleep(Duration::from_secs(1));

            let form = Form::new()
                .text("login", config.username)
                .text("password", config.password)
                .text("remember", "1")
                .text("_xfToken", xf_token);

            client
                .post("https://forum.questionablequesting.com/login/login")
                .multipart(form)
                .send()?
                .text()?;

            thread::sleep(Duration::from_secs(1));
            Channel::read_from(Cursor::new(client.get(&url).send()?.text()?))?
        }
    };

    {
        let mut f = File::create(&config.cookie_jar)?;
        let store = cookie_store.lock().unwrap();
        store.save_json(&mut f).unwrap();
    }

    // Fix the link to the thread
    feed.set_link(format!("https://forum.questionablequesting.com/threads/{thread_id}"));

    println!("{}", feed.to_string());

    Ok(())
}
