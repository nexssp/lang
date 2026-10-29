//! ```cargo
//! [dependencies]
//! serde_json = "1"
//! ```

use serde_json::Value;
use std::io::{self, Read};

fn main() {
    let mut raw = String::new();
    io::stdin().read_to_string(&mut raw).expect("read JSON stdin");
    let input: Value = serde_json::from_str(&raw).expect("parse JSON stdin");
    let output = input; // TODO: transform input
    println!("{}", output);
}
