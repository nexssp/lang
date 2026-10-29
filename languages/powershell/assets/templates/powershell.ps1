$raw = [Console]::In.ReadToEnd()
if ([string]::IsNullOrWhiteSpace($raw)) { $raw = "null" }
$inputValue = ConvertFrom-Json -InputObject $raw
$outputValue = $inputValue # TODO: transform input
ConvertTo-Json -InputObject $outputValue -Compress -Depth 100
