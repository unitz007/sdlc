use std::fs::*;
use std::process::{Command, Stdio};
use std::io::{Error, BufReader, ErrorKind, BufRead};
use std::env;
use std::borrow::Borrow;
use sdlc::model::{Build};

fn main() -> Result<(), Error> {
    let program = env::current_exe().unwrap();

    let program_name = program.file_name().unwrap();

    let program_full_path = env::current_exe()
        .unwrap()
        .as_path()
        .to_str()
        .unwrap()
        .to_string();

    let program_path = program_full_path.split_at(program_full_path.len() - program_name.len()).0;

    let contents = sdlc::read_json(program_path.borrow());

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
                com = sdlc::get_task(&arg, &command.tasks);
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
