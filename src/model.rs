use serde::{Deserialize, Serialize};

#[derive(Debug, Deserialize, Serialize)]
pub struct Tasks {
    pub program: String,
    pub run: String,
    pub test: String,
    pub build: String,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Commands {
    pub build_file: String,
    pub tasks: Tasks,
}

#[derive(Debug, Deserialize, Serialize)]
pub struct Build {
    pub builds: Vec<Commands>,
}
