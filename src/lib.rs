pub mod model;

use std::fs::*;
use std::io::Read;

pub fn read_json(path: &str) -> String {

    let mut file = File::open(path.to_string() + "file.json").unwrap();

    let mut json_content = String::new();

    file.read_to_string(&mut json_content).expect("error reading file content");

    return json_content;
}


pub fn get_task<'a>(arg: &'a String, task: &'a model::Tasks) -> &'a str {
    if arg.eq("run") {
        &task.run
    } else if arg.eq("test") {
        &task.test
    } else if arg.eq("build") {
        &task.build
    } else {
        panic!("Invalid sdlc command");
    }
}