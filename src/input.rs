use std::path::{Path, PathBuf};

use crate::{config, filters, logging, mkv, select};
use json::JsonValue;

pub fn process(config: &Path, input: &Path, working: &Path, output: &Path) -> Result<(), ()> {
	let cfg = config::load(config);

	for entry in cfg {
		let name = entry.0;
		let data = entry.1;

		let path = input.join(&name);
		if !path.exists() || !path.is_dir() {
			continue;
		}

		let wdir = working.join(&name);
		let odir = output.join(&name);

		let err = std::fs::create_dir_all(&wdir);
		if let Err(err) = err {
			let name = wdir.to_str().unwrap();

			logging::error!("Failed to create directory {}: {}", name, err);
			return Err(());
		}

		let err = std::fs::create_dir_all(&odir);
		if let Err(err) = err {
			let name = odir.to_str().unwrap();

			logging::error!("Failed to create directory {}: {}", name, err);
			return Err(());
		}

		logging::scope("config", &name, || process_dir(&data, &path, &wdir, &odir))?;
	}

	Ok(())
}

fn process_dir(cfg: &JsonValue, dir: &Path, working: &Path, output: &Path) -> Result<(), ()> {
	let files = std::fs::read_dir(dir);
	if let Err(err) = files {
		logging::error!("Failed to list files in directory: {}", err);
		return Err(());
	}

	for entry in std::fs::read_dir(dir).expect("read_dir failed") {
		let path = entry.unwrap().path();
		let ext = path.extension().unwrap_or_default();

		// We only care about mkv files and directories
		if !(path.is_dir() || path.is_file() && ext == "mkv") {
			continue;
		}

		let name = path.file_stem().unwrap_or_default().to_str();
		if name.is_none() {
			continue;
		}

		let name = String::from(name.unwrap());
		let wdir = working.join(&name);
		let ofile = output.join(&name).with_extension("mkv");

		let err = std::fs::create_dir_all(&wdir);
		if let Err(err) = err {
			let name = wdir.to_str().unwrap();

			logging::error!("Failed to create directory {}: {}", name, err);
			return Err(());
		}

		logging::scope("item", &name, || process_item(cfg, &path, &wdir, &ofile))?;
	}

	Ok(())
}

fn process_item(cfg: &JsonValue, path: &Path, working: &Path, output: &Path) -> Result<(), ()> {
	let mut files = Vec::<PathBuf>::new();

	if output.exists() {
		return Ok(());
	}

	// Build a list of all mkv files related to the current item
	if path.is_file() {
		files.push(path.to_path_buf());
	} else {
		for entry in std::fs::read_dir(path).expect("read_dir failed") {
			let p2 = entry.unwrap().path();
			let ext = p2.extension().unwrap_or_default();

			if !p2.is_file() || ext != "mkv" {
				continue;
			}

			files.push(p2);
		}
	}

	// Normalize the metadata of the input files
	let files = mkv::normalize(&files, working)?;

	// Probe all streams of the input files
	let streams = mkv::streams(&files)?;

	// Select the input streams we care about
	let streams = select::find(cfg, &streams).unwrap();

	// Store the processed streams
	let mut processed = Vec::<mkv::Stream>::new();

	// Run processing filters
	for entry in streams {
		let cfg = entry.0;
		let stream = entry.1;

		let stream = logging::scope("stream", &stream.id, || {
			filters::run(&cfg, &stream, working)
		})?;
		processed.push(stream);
	}

	// Combine the processed streams into a new mkv
	mkv::write(&processed, output)
}
