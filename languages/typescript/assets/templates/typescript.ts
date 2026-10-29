import { readFileSync } from "node:fs";

const input: unknown = JSON.parse(readFileSync(0, "utf8"));
const output: unknown = input; // TODO: transform input
console.log(JSON.stringify(output));
