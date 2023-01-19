const fs = require('fs')
const path = require('path')
const spawn = require('child_process').spawn
const process = require('process')
let filename = process.argv[1].split('\\')
filename = filename[filename.length - 1]
filename = filename.split('.')[0]
const project = filename
function walkSync(currentDirPath, callback) {
	var fs = require('fs')
	fs.readdirSync(currentDirPath, { withFileTypes: true }).forEach(function (dirent) {
		var filePath = path.join(currentDirPath, dirent.name)
		if (dirent.isFile()) {
			callback(filePath, dirent)
		} else if (dirent.isDirectory()) {
			walkSync(filePath, callback)
		}
	})
}
let filelist = []
walkSync('./', function (filePath, stat) {
	if (filePath.indexOf('.go') > 0 || filePath.indexOf('.yaml') > 0) {
		let data = {
			path: filePath,
			ctimeMs: fs.statSync(filePath).ctimeMs,
		}
		filelist.push(data)
	}
})
let child = spawn('go', ['run', `${project}.go`])
child.stdout.setEncoding('utf8')
child.stderr.setEncoding('utf8')
child.stderr.on('data', function (data) {
	process.stdout.write(data)
})
child.stdout.on('data', function (data) {
	process.stdout.write(data)
})
setInterval(() => {
	for (let i = 0; i < filelist.length; i++) {
		if (filelist[i].ctimeMs != fs.statSync(filelist[i].path).ctimeMs) {
			fs.appendFileSync(`${project}.js`, ' ')
			break
		}
	}
}, 200)
