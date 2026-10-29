import json
import sys

input_value = json.load(sys.stdin)
output_value = input_value  # TODO: transform input
json.dump(output_value, sys.stdout, separators=(",", ":"))
sys.stdout.write("\n")
