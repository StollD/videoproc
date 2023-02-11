use json::JsonValue;

use crate::{logging, mkv, utils};

pub fn find(
	cfg: &JsonValue,
	streams: &Vec<mkv::Stream>,
) -> Result<Vec<(String, JsonValue, mkv::Stream)>, ()> {
	let mut new = Vec::<(String, JsonValue, mkv::Stream)>::new();

	for entry in cfg.entries() {
		let name = String::from(entry.0);
		let streamcfg = entry.1;

		let m = find_match(streamcfg, streams);
		if m.is_none() {
			logging::error!("Could not find match for stream {}", name);
			return Err(());
		}

		let (j, s) = m.unwrap();
		new.push((name, j, s));
	}

	new.sort_by(|a, b| {
		let x = utils::streampriority(&a.2);
		let y = utils::streampriority(&b.2);

		x.cmp(&y)
	});

	Ok(new)
}

fn find_match(cfg: &JsonValue, streams: &Vec<mkv::Stream>) -> Option<(JsonValue, mkv::Stream)> {
	for option in cfg.members() {
		for stream in streams {
			if !check_match(option, stream) {
				continue;
			}

			return Some((option.clone(), stream.clone()));
		}
	}

	None
}

fn check_match(cfg: &JsonValue, stream: &mkv::Stream) -> bool {
	if cfg.has_key("type") && cfg["type"] != stream.streamtype.as_str() {
		return false;
	}

	if cfg.has_key("lang") && cfg["lang"] != stream.language.as_deref().unwrap_or("None") {
		return false;
	}

	if cfg.has_key("id") && !cfg["id"].contains(stream.id.as_str()) {
		return false;
	}

	if cfg.has_key("file") {
		let mut m = false;
		let path = stream.path.to_str().unwrap_or_default().to_string();

		for entry in cfg["file"].members() {
			let regex = fnmatch_regex::glob_to_regex(format!("*{entry}*").as_str());
			if regex.is_err() {
				continue;
			}

			let regex = regex.unwrap();
			if regex.is_match(&path) {
				m = true;
			}
		}

		if !m {
			return false;
		}
	}

	true
}
