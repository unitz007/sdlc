// use structopt::StructOpt;
use std::fs::*;
use std::path::Path;
use serde::{Deserialize, Serialize};
use std::process::{Command, Stdio};
use std::io::{Error, BufReader, ErrorKind, BufRead};
// use std::io::{_Write};


fn main()-> Result<(), Error> {
    let contents = read_json("file.json");

    let current_directory = std::env::current_dir().expect("error accessing directory");

    // println!("{:?}", current_directory);

    let build: Build = serde_json::from_str(&contents).unwrap();

    let mut program = "";
    let mut com = "";


    for command in build.builds.iter() {
        let build_file = &command.build_file;



        for f in read_dir(&current_directory).unwrap().into_iter() {
            let file = f.unwrap().file_name().into_string().unwrap();
            if file.eq(build_file) {
                // break;
                // println!("{}", build_file);
                program = &command.tasks.program;
                com = &command.tasks.run;
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

// print!("{}", program);


fn read_json(path: &str) -> String {

    // println!("{:?}", Path::new(path));
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

