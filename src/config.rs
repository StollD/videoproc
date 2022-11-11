use std::{
	collections::BTreeMap,
	path::{Path, PathBuf},
};

use json::{array, object, JsonValue};

use crate::logging;

pub fn load(config: &Path) -> BTreeMap<String, JsonValue> {
	let mut result = BTreeMap::<String, JsonValue>::new();

	for entry in config.read_dir().expect("read_dir failed") {
		let path = entry.unwrap().path();

		if !path.is_file() {
			continue;
		}

		if path.extension().unwrap_or_default() != "json" {
			continue;
		}

		let data = load_dict(&path);
		if data.is_err() {
			continue;
		}

		let data = data.unwrap();

		let data = resolve_dict(config, &data);
		if data.is_err() {
			logging::error!("Failed to resolve links for {}!", path.to_str().unwrap());
			continue;
		}

		let data = data.unwrap();

		let name = path.file_stem().unwrap_or_default().to_str();
		if name.is_none() {
			continue;
		}

		result.insert(String::from(name.unwrap()), data);
	}

	result
}

fn load_dict(path: &Path) -> Result<JsonValue, ()> {
	let data = std::fs::read_to_string(path);
	if data.is_err() {
		logging::error!("Could not read file {}!", path.to_str().unwrap());
		return Err(());
	}

	let data = json::parse(data.unwrap().as_str());
	if data.is_err() {
		logging::error!("Could not parse file {}!", path.to_str().unwrap());
		return Err(());
	}

	let data = data.unwrap();
	if !data.is_object() {
		logging::error!("Configuration {} is invalid!", path.to_str().unwrap());
		return Err(());
	}

	Ok(data)
}

fn resolve_dict(config: &Path, value: &JsonValue) -> json::Result<JsonValue> {
	let mut new = object! {};

	for entry in value.entries() {
		let k = entry.0;
		let v = entry.1;

		if v.is_object() {
			new[k] = resolve_dict(config, v)?;
		} else if v.is_array() && k != "$link" {
			new[k] = resolve_list(config, v)?;
		} else {
			let data = resolve_value(config, k, v)?;

			for e2 in data.entries() {
				new[e2.0] = e2.1.clone();
			}
		}
	}

	Ok(new)
}

fn resolve_list(config: &Path, value: &JsonValue) -> json::Result<JsonValue> {
	let mut new = array![];

	for entry in value.members() {
		if entry.is_object() {
			new.push(resolve_dict(config, entry)?)?;
		} else if entry.is_array() {
			new.push(resolve_list(config, entry)?)?;
		} else {
			new.push(entry.clone())?;
		}
	}

	Ok(new)
}

fn resolve_value(config: &Path, key: &str, value: &JsonValue) -> json::Result<JsonValue> {
	let mut new = object! {};

	if key == "$link" {
		let paths = if !value.is_array() {
			array![value.clone()]
		} else {
			value.clone()
		};

		for path in paths.members() {
			if !path.is_string() {
				continue;
			}

			let path = config.join(path.as_str().unwrap());

			let data = load_dict(&path);
			if data.is_err() {
				continue;
			}

			let data = data.unwrap();
			let data = resolve_dict(config, &data)?;

			for entry in data.entries() {
				new[entry.0] = entry.1.clone();
			}
		}
	} else {
		let mut nv = value.clone();

		if nv.is_string() {
			let s = nv.as_str().unwrap_or_default();

			// If a value references the config directory, replace the path and make it absolute
			if s.contains("$(configs)$") {
				let s = s.replace("$(configs)$", config.to_str().unwrap());

				let path = PathBuf::from(s);
				let path = match path.canonicalize() {
					Ok(val) => val,
					Err(_) => path,
				};

				nv = JsonValue::from(path.to_str().unwrap());
			}
		}

		new[key] = nv;
	}

	Ok(new)
}
