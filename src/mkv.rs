use execute::Execute;
use json::object;

use crate::{
	logging,
	utils::{self, framerate, StrVec},
};
use std::{
	io,
	path::{Path, PathBuf},
	process::Command,
};

#[derive(Clone)]
pub struct Stream {
	pub path: PathBuf,
	pub index: i32,
	pub streamtype: String,
	pub id: String,
	pub language: Option<String>,
	pub offset: f32,
	pub duration: f32,
	pub codec: Option<String>,

	pub aspect: Option<String>,
	pub framerate: Option<(u32, u32)>,

	pub samplerate: Option<u32>,
	pub channels: Option<u32>,
	pub dialnorm: Option<i32>,
	pub dsurmode: Option<u32>,
}

impl Stream {
	pub fn load(path: &Path) -> Result<Self, ()> {
		let data = std::fs::read_to_string(path);
		if let Err(err) = data {
			let name = path.to_str().unwrap();

			logging::error!("Failed to read {}: {}", name, err);
			return Err(());
		}

		let data = json::parse(&data.unwrap());
		if let Err(err) = data {
			let name = path.to_str().unwrap();

			logging::error!("Failed to parse {}: {}", name, err);
			return Err(());
		}

		let mut data = data.unwrap();

		let stream = Self {
			path: PathBuf::from(data["path"].to_string()),
			index: data["index"].as_i32().unwrap(),
			streamtype: data["streamtype"].to_string(),
			id: data["id"].to_string(),
			language: data["language"].take_string(),
			codec: data["codec"].take_string(),
			offset: data["offset"].as_f32().unwrap(),
			duration: data["duration"].as_f32().unwrap(),
			aspect: data["aspect"].take_string(),
			framerate: if data["framerate"].is_null() {
				None
			} else {
				Some(framerate(&data["framerate"].to_string()))
			},
			samplerate: data["samplerate"].as_u32(),
			channels: data["channels"].as_u32(),
			dialnorm: data["dialnorm"].as_i32(),
			dsurmode: data["dsurmode"].as_u32(),
		};

		Ok(stream)
	}

	pub fn save(&self, path: &Path) -> io::Result<()> {
		let framerate = if self.framerate.is_some() {
			let f = self.framerate.unwrap();
			Some(format!("{}/{}", f.0, f.1))
		} else {
			None
		};

		let obj = object! {
			path: self.path.to_str().unwrap(),
			index: self.index,
			streamtype: self.streamtype.as_str(),
			id: self.id.as_str(),
			language: self.language.as_deref(),
			codec: self.codec.as_deref(),
			offset: self.offset,
			duration: self.duration,
			aspect: self.aspect.as_deref(),
			framerate: framerate,
			samplerate: self.samplerate,
			channels: self.channels,
			dialnorm: self.dialnorm,
			dsurmode: self.dsurmode,
		};

		let str = json::stringify_pretty(obj, 4);
		std::fs::write(path, str)
	}

	pub fn cleanup(&self) -> io::Result<()> {
		if self.path.exists() {
			return std::fs::remove_file(&self.path);
		}

		Ok(())
	}
}

pub fn normalize(files: &Vec<PathBuf>, working: &Path) -> Result<Vec<PathBuf>, ()> {
	let mut new = Vec::<PathBuf>::new();

	logging::info!("Normalizing metadata");

	for file in files {
		let stem = file.file_stem().unwrap_or_default();

		let wdir = if files.len() > 1 {
			working.join(stem)
		} else {
			working.to_path_buf()
		};

		let err = std::fs::create_dir_all(&wdir);
		if let Err(err) = err {
			let name = wdir.to_str().unwrap();

			logging::error!("Failed to create directory {}: {}", name, err);
			return Err(());
		}

		new.push(normalize_file(file, &wdir)?);
	}

	Ok(new)
}

fn normalize_file(file: &Path, working: &Path) -> Result<PathBuf, ()> {
	let name = working.file_name().unwrap_or_default();
	let output = working.join(name).with_extension("norm.mkv");

	if output.exists() {
		return Ok(output);
	}

	// Normalize MKV metadata by remuxing the file with mkvmerge
	let temp = output.with_extension("temp.mkv");
	let cmd = Command::new("mkvmerge")
		.arg("-o")
		.arg(&temp)
		.arg(file)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		let name = file.to_str().unwrap();

		logging::error!("Failed to normalize {}: {}", name, err);
		return Err(());
	}

	let err = std::fs::rename(&temp, &output);
	if let Err(err) = err {
		let name = temp.to_str().unwrap();

		logging::error!("Failed to rename {}: {}", name, err);
		return Err(());
	}

	Ok(output)
}

pub fn streams(files: &Vec<PathBuf>) -> Result<Vec<Stream>, ()> {
	let mut streams = Vec::<Stream>::new();

	for file in files {
		if !file.exists() {
			let name = file.to_str().unwrap();

			logging::error!("Failed to probe {}", name);
			return Err(());
		}

		let cmd = Command::new("ffprobe")
			.arg(file)
			.arg("-of")
			.arg("json")
			.arg("-show_format")
			.arg("-show_chapters")
			.arg("-probesize")
			.arg("10G")
			.arg("-analyzeduration")
			.arg("10G")
			.output();

		let cmd = utils::check_output(cmd);
		if let Err(err) = cmd {
			let name = file.to_str().unwrap();

			logging::error!("Failed to probe {}: {}", name, err);
			return Err(());
		}

		let data = String::from_utf8(cmd.unwrap().stdout);
		if let Err(err) = data {
			let name = file.to_str().unwrap();

			logging::error!("Failed to decode probe of {}: {}", name, err);
			return Err(());
		}

		let data = json::parse(&data.unwrap());
		if let Err(err) = data {
			let name = file.to_str().unwrap();

			logging::error!("Failed to decode probe of {}: {}", name, err);
			return Err(());
		}

		let data = data.unwrap();
		let count = data["format"]["nb_streams"].as_u32().unwrap_or_default();

		for i in 0..count {
			streams.push(stream(file, i)?);
		}

		if data["chapters"].is_empty() {
			continue;
		}

		let chapters = Stream {
			path: file.clone(),
			index: -1,
			streamtype: String::from("chapters"),
			id: String::from("chapters"),
			language: None,
			codec: None,
			offset: 0.0,
			duration: 0.0,
			aspect: None,
			framerate: None,
			samplerate: None,
			channels: None,
			dialnorm: None,
			dsurmode: None,
		};

		streams.push(chapters);
	}

	Ok(streams)
}

pub fn stream(file: &Path, index: u32) -> Result<Stream, ()> {
	if !file.exists() {
		let name = file.to_str().unwrap();

		logging::error!("Failed to probe {}", name);
		return Err(());
	}

	let cmd = Command::new("ffprobe")
		.arg(file.to_str().unwrap())
		.arg("-of")
		.arg("json")
		.arg("-show_streams")
		.arg("-probesize")
		.arg("10G")
		.arg("-analyzeduration")
		.arg("10G")
		.output();

	let cmd = utils::check_output(cmd);

	if let Err(err) = cmd {
		let name = file.to_str().unwrap();

		logging::error!("Failed to probe {}: {}", name, err);
		return Err(());
	}

	let data = String::from_utf8(cmd.unwrap().stdout);
	if let Err(err) = data {
		let name = file.to_str().unwrap();

		logging::error!("Failed to decode probe of {}: {}", name, err);
		return Err(());
	}

	let data = json::parse(&data.unwrap());
	if let Err(err) = data {
		let name = file.to_str().unwrap();

		logging::error!("Failed to decode probe of {}: {}", name, err);
		return Err(());
	}

	let cmd = Command::new("mediainfo")
		.arg(file)
		.arg("-F")
		.arg("--Output=JSON")
		.output();

	let cmd = utils::check_output(cmd);
	if let Err(err) = cmd {
		let name = file.to_str().unwrap();

		logging::error!("Failed to run mediainfo on {}: {}", name, err);
		return Err(());
	}

	let info = String::from_utf8(cmd.unwrap().stdout);
	if let Err(err) = info {
		let name = file.to_str().unwrap();

		logging::error!("Failed to decode mediainfo of {}: {}", name, err);
		return Err(());
	}

	let info = json::parse(&info.unwrap());
	if let Err(err) = info {
		let name = file.to_str().unwrap();

		logging::error!("Failed to decode mediainfo of {}: {}", name, err);
		return Err(());
	}

	let data = data.unwrap();
	let info = info.unwrap();

	for entry in data["streams"].members() {
		if entry["index"].as_u32().unwrap_or_default() != index {
			continue;
		}

		let mediainfo = &info["media"]["track"][index as usize + 1];

		let streamtype = entry["codec_type"].to_string();
		let codec = Some(entry["codec_name"].to_string());
		let offset = entry["start_time"]
			.to_string()
			.parse::<f32>()
			.unwrap_or_default();

		let tags = &entry["tags"];

		let id = if tags.has_key("SOURCE_ID") {
			tags["SOURCE_ID"].to_string()
		} else {
			format!("{}:{}", streamtype, index)
		};

		let language = if tags.has_key("language") {
			Some(tags["language"].to_string())
		} else {
			None
		};

		let duration = if tags.has_key("DURATION") {
			let dur = tags["DURATION"].to_string();
			let mut split = dur.split(':');

			let hours = split.next().unwrap().parse::<f32>().unwrap_or_default();
			let minutes = split.next().unwrap().parse::<f32>().unwrap_or_default();
			let seconds = split.next().unwrap().parse::<f32>().unwrap_or_default();

			seconds + minutes * 60.0 + hours * 60.0 * 60.0
		} else if entry.has_key("duration") {
			let dur = entry["duration"].to_string();

			dur.parse::<f32>().unwrap_or_default()
		} else {
			0.0
		};

		let aspect = if streamtype == "video" {
			Some(entry["display_aspect_ratio"].to_string())
		} else {
			None
		};

		let framerate = if streamtype == "video" {
			Some(utils::framerate(&entry["avg_frame_rate"].to_string()))
		} else {
			None
		};

		let samplerate = if streamtype == "audio" {
			mediainfo["SamplingRate"].to_string().parse::<u32>().ok()
		} else {
			None
		};

		let channels = if streamtype == "audio" {
			entry["channels"].as_u32()
		} else {
			None
		};

		let dialnorm = if streamtype == "audio" {
			let extra = &mediainfo["extra"];

			if extra.has_key("dialnorm_Average") {
				extra["dialnorm_Average"].to_string().parse::<i32>().ok()
			} else {
				None
			}
		} else {
			None
		};

		let dsurmode = if streamtype == "audio" {
			let extra = &mediainfo["extra"];

			if extra.has_key("dsurmod") {
				extra["dsurmod"].to_string().parse::<u32>().ok()
			} else {
				None
			}
		} else {
			None
		};

		let stream = Stream {
			path: file.to_path_buf(),
			index: index as i32,
			streamtype,
			id,
			language,
			codec,
			offset,
			duration,
			aspect,
			framerate,
			samplerate,
			channels,
			dialnorm,
			dsurmode,
		};

		return Ok(stream);
	}

	Err(())
}

pub fn write(streams: &Vec<Stream>, path: &Path) -> Result<(), ()> {
	let mut args = Vec::<String>::new();
	let mut chapters = Vec::<&Stream>::new();
	let mut choffset: f32 = 0.0;

	let temp = path.with_extension("temp.mkv");

	for stream in streams {
		if stream.streamtype == "chapters" {
			chapters.push(stream);
		} else {
			args.push_str("-itsoffset");
			args.push(format!("{}s", &stream.offset));

			args.push_str("-i");
			args.push(stream.path.to_str().unwrap().to_string());

			choffset = choffset.min(stream.offset);
		}
	}

	// If one of the streams has a negative offset, ffmpeg will instead push all other streams
	// forward, because negative timestamps are not supported. However, chapters will not be
	// affected, so this shift needs to be applied manually.
	for chap in chapters {
		let mut offset = chap.offset;
		if choffset < 0.0 {
			offset -= choffset
		}

		args.push_str("-itsoffset");
		args.push(format!("{}s", offset));

		args.push_str("-i");
		args.push(chap.path.to_str().unwrap().to_string());
	}

	for (i, stream) in streams.iter().enumerate() {
		if stream.streamtype == "chapters" {
			args.push_str("-map_chapters");
			args.push(i.to_string());
			continue;
		}

		args.push_str("-map");
		args.push(format!("{}:{}", i, stream.index));

		// Unset some metadata
		let meta = format!("-metadata:s:{}", i);
		args.push(meta.clone());
		args.push_str("title=");
		args.push(meta.clone());
		args.push_str("SOURCE_ID=");
		args.push(meta.clone());
		args.push_str("ENCODER=");

		args.push(meta.clone());

		// Set language metadata
		if stream.streamtype == "video" {
			args.push_str("language=");
		} else {
			args.push(format!(
				"language={}",
				stream.language.as_deref().unwrap_or_default()
			));
		}
	}

	args.push_str("-codec");
	args.push_str("copy");

	args.push_str("-disposition");
	args.push_str("0");

	args.push_str("-disposition:a:0");
	args.push_str("default");

	args.push_str("-metadata");
	args.push_str("title=");

	args.push_str("-y");
	args.push_str(temp.to_str().unwrap());

	let cmd = Command::new("ffmpeg")
		.args(args)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run ffmpeg: {}", err);
		return Err(());
	}
	let cmd = Command::new("mkvmerge")
		.arg("-o")
		.arg(path)
		.arg(&temp)
		.execute_check_exit_status_code(0);

	if let Err(err) = cmd {
		logging::error!("Failed to run mkvmerge: {}", err);
		return Err(());
	}

	let err = std::fs::remove_file(temp);
	if let Err(err) = err {
		logging::error!("Failed to remove file: {}", err);
		return Err(());
	}

	Ok(())
}
