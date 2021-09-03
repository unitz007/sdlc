use std::fs::*;
use serde::{Deserialize, Serialize};
use std::process::{Command, Stdio};
use std::io::{Error, BufReader, ErrorKind, BufRead, Read};
use std::env;
use std::borrow::Borrow;


fn main()-> Result<(), Error> {
    let program = env::current_exe().unwrap();

    let program_name = program.file_name().unwrap();

    let program_full_path = env::current_exe()
        .unwrap()
        .as_path()
        .to_str()
        .unwrap()
        .to_string();

    let program_path = program_full_path.split_at(program_full_path.len() - program_name.len()).0;

    let contents = read_json(program_path.borrow());

    let args: Vec<String>= env::args().skip(1).collect();

    if args.is_empty() {
        panic!("Please provide an argument");
    }

    let current_directory = std::env::current_dir().expect("error accessing directory");

    let build: Build = serde_json::from_str(&contents).unwrap();

    let mut program = "";
    let mut com = "";

    for command in build.builds.iter() {
        let build_file = &command.build_file;

        for f in read_dir(&current_directory).unwrap().into_iter() {
            let file = f.unwrap().file_name().into_string().unwrap();
            if file.eq(build_file) {
                program = &command.tasks.program;
                let arg = &args[0];
                com = get_task(&arg, &command.tasks);
            }
        }
    }

    let output = Command::new(program.to_string())
        .args(&[com.to_string()])
        .stdout(Stdio::piped())
        .spawn()?
        .stdout
        .ok_or_else(|| Error::new(ErrorKind::Other,"Could not capture standard output."))?;

    let reader = BufReader::new(output);

    reader.lines()
        .filter_map(|line| line.ok())
        .for_each(|line| println!("{}", line));

    Ok(())

}

fn read_json(path: &str) -> String {

    let mut file = File::open(path.to_string() + "file.json").unwrap();

    let mut json_content = String::new();

    file.read_to_string(&mut json_content).expect("error reading file content");

    return json_content;
}


#[derive(Debug, Deserialize, Serialize)]
struct Tasks {
    program: String,
    run: String,
    test: String,
    build: String,
}

#[derive(Debug, Deserialize, Serialize)]
struct Commands {
    build_file: String,
    tasks: Tasks,
}

#[derive(Debug, Deserialize, Serialize)]
struct Build {
    builds: Vec<Commands>,
}

fn get_task<'a>(arg: &'a String, task: &'a Tasks) -> &'a str {
    if  arg.eq("run") {
        &task.run
    } else if arg.eq("test") {
        &task.test
    } else if arg.eq("build") {
        &task.build
    } else {
        panic!("Invalid sdlc command");
    }

}

