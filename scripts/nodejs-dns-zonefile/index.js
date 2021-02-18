#!/usr/bin/env node
const fs = require('fs');
const util = require('util');
const render = require('template-file');

const args = process.argv.slice(2);

if (args.length != 1) {
    console.error("please set template file as 1st argument. Will read from stdin and write to stdout.");
    process.exit(1);
}

var templateBuffer = fs.readFileSync(args[0]);

var stdinBuffer = fs.readFileSync(0);
if (stdinBuffer.length <= 0) {
    console.error("no data")
    process.exit(2);
}
const obj = JSON.parse(stdinBuffer);

// restructure 

let data = {
    serial: obj.lastUpdate,
    a_records: [],
    cname_records: [],
    txt_records: []
}

var all_names = [];
var all_ips = [];
for ( const key in obj.members) {
    nodeName = key
    nodeElem = obj.members[key]

    data.a_records.push({ 
        name: nodeName,
        ip: nodeElem.addr
    })
    all_names.push(nodeName)
    all_ips.push(nodeElem.addr)
}

if (all_ips.length > 0) {
    data.a_records.push({
        name: "all",
        ip: all_ips.reduce((acc, ip) => {
            return ""+acc+" "+ip;
        }, "")
    })
   
}

if (obj.services !== undefined) {
    for ( const key in obj.services) {
        nodeName = key
        nodeElem = obj.services[key]
    
        data.cname_records.push({ 
            cname: nodeName,
            names: nodeElem.nodes.reduce((acc, n) => {
                return ""+acc+" "+n;
            }, "")
        })
        data.txt_records.push({ 
            name: nodeName,
            text: util.format("port=%s",nodeElem.port),
        })
    }
    
}

console.log(render.render(templateBuffer.toString(), data))