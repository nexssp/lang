(async () => {
    let raw;
    if (typeof Deno !== "undefined") {
        raw = await new Response(Deno.stdin.readable).text();
    } else {
        raw = require("node:fs").readFileSync(0, "utf8");
    }
    const input = JSON.parse(raw);
    const output = input; // TODO: transform input
    console.log(JSON.stringify(output));
})();
