# <img src="./icon.png" alt="Icon" style="height: 36px; width: auto; vertical-align: text-top; margin-right: 6px">Logseq Tools



Welcome to Logseq Tools, a workflow to enhance your note-taking experience in Logseq. Easily search note names, tags, and list notes containing specific tags.

## Features

1. Utilizes Logseq's official [Local HTTP server](https://docs.logseq.com/#/page/local%20http%20server) , ensuring data stays local and privacy is protected.
2. Offers full-text search for note titles, including Chinese Pinyin search support.
3. Supports note Tag search.


## Note

Logseq Tools uses libsimple.dylib from [Simple tokenizer](https://github.com/wangfenjin/simple) for full-text search. Due to Apple's security restrictions, allow the library's usage in Security Settings upon first use. Go to System Preferences > Security & Privacy > General and click "Allow" next to the libsimple.dylib message.

## Configuration

Before using Logseq Tools, generate a Logseq API Token and set up the token in the workflow settings.

To generate a token, go to Logseq Settings > Features, enable "HTTP APIs server," return to the main interface, click the "API" button, select "Authorization tokens," and generate a token.

Before using Logseq Tools, generate a note index by typing `refreshlogseq`. Update the index with `refreshlogseq` whenever notes are updated.

## Usage

| Command       | Description                                 |
|---------------|---------------------------------------------|
| lsn           | Search note names                           |
| lst           | Search tags                                 |
| lsnt          | List notes with specific tags              |
| refreshlogseq | Generate note index by refreshing database |

## How It Works

Logseq Tools operates by utilizing Logseq's [Local HTTP server](https://docs.logseq.com/#/page/local%20http%20server) to obtain a comprehensive list of Logseq note metadata. The workflow then scans this list using note names and tags to identify pertinent notes.

To facilitate full-text filename search in Chinese with Pinyin, the [Simple tokenizer](https://github.com/wangfenjin/simple) is also integrated.


## TODO

- [ ] support quick capture
