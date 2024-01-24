use clap::Parser;

mod ao3;
mod jnovel;

#[derive(Debug, Parser)]
#[clap(
    name = "rss-scrapers",
    about = "Tool for scraping various different sites and constructing rss feeds"
)]
pub struct Opt {
    // #[arg(short, long, value_parser)]
    /// Override the selected config.
    // awconf: Option<PathBuf>,

    #[command(subcommand)]
    cmd: Command,
}

#[derive(Debug, Parser)]
enum Command {
    /// Archive Of Our Own
    Ao3 {
        /// The story id from the URL.
        /// https://archiveofourown.org/works/1234 has a story id of 1234
        #[arg(allow_hyphen_values = true)]
        story_id: String,
    },
    /// Jnovel-club series
    Jnovel {
        /// The jnovel title slug, from after /series/ in the title.
        /// https://j-novel.club/series/ab-cd has a title slug of ab-cd
        #[arg(allow_hyphen_values = true)]
        title_slug: String,
    },
}


// #[derive(Debug, Deserialize)]
// pub struct Credentials {
//     #[serde(default)]
//     xx_username: String,
//     #[serde(default)]
//     xx_password: String,
// }

fn main() {
    let opt = Opt::parse();

    match opt.cmd {
        Command::Ao3 { story_id } => {
            ao3::get(story_id).unwrap();
        }
        Command::Jnovel { title_slug } => {
            jnovel::get(title_slug).unwrap();
        }
    }
}
