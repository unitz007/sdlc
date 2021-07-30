// use structopt::StructOpt;
use std::fs::*;
use std::path::Path;
use serde::{Deserialize, Serialize};
use std::ffi::OsString;
use std::io::Error;
use std::process::Command as command;

fn main() {
    let contents = read_json("file.json");

    let current_directory = std::env::current_dir().expect("error accessing directory");

    let files = read_dir(current_directory);

    let build: Build = serde_json::from_str(&contents).unwrap();

    let list_directory = command::new("ls").output().expect("Error executing command");

    for command in build.builds.iter() {
        // let e =  std::str::from_utf8(list_directory.stdout.as_slice()).unwrap().to_string().contains(command.build_file);
    }



fn read_json(path: &str) -> String {
    let file = File::open(Path::new(path));
    let contents;

    match file {
        Ok(_) => {
            contents = read_to_string(Path::new(path)).expect("error while reading file content");
        }
        Err(_) => {
            File::create(path).expect("error creating file");
            contents = read_to_string(Path::new(path)).expect("error reading file content")
        }
    }

    return contents;
}

#[derive(Debug, Deserialize, Serialize)]
struct Tasks {
    run : String,
    test: String,
    build: String
}

#[derive(Debug, Deserialize, Serialize)]
struct Commands {
    build_file: String,
    tasks: Tasks
}

#[derive(Debug, Deserialize, Serialize)]
struct Build {
    builds: Vec<Commands>
}

