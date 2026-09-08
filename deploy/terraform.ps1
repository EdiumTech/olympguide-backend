param(
    [ValidateSet('init', 'validate', 'plan', 'apply', 'output')]
    [string]$Action = 'plan'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$terraformDir = Join-Path $projectRoot 'infra/yandex'
$terraformExe = Join-Path $projectRoot '.tools/terraform.exe'
$ycExe = Join-Path $projectRoot '.tools/yc.exe'
if (-not (Test-Path -LiteralPath $terraformExe)) { $terraformExe = (Get-Command terraform).Source }
$previousConfig = $env:TF_CLI_CONFIG_FILE
$previousToken = $env:YC_TOKEN
$env:TF_CLI_CONFIG_FILE = Join-Path $terraformDir 'terraform.rc'
try {
    if ($Action -in @('plan', 'apply') -and -not $env:YC_TOKEN -and -not $env:YC_SERVICE_ACCOUNT_KEY_FILE) {
        if (-not (Test-Path -LiteralPath $ycExe)) { $ycExe = (Get-Command yc).Source }
        $ycConfig = Join-Path $projectRoot '.private/yc-config.yaml'
        $ycArgs = @('iam', 'create-token')
        if (Test-Path -LiteralPath $ycConfig) { $ycArgs += @('--config', $ycConfig) }
        $issuedToken = & $ycExe @ycArgs
        if ($LASTEXITCODE -ne 0) { throw 'Authorize yc before running Terraform.' }
        # yc may print an interactive sign-in notice before the token.
        $issuedToken = ($issuedToken | Select-Object -Last 1).Trim()
        if (-not $issuedToken -or $issuedToken -match '\s') { throw 'yc did not return a valid token line.' }
        $env:YC_TOKEN = $issuedToken
    }
    switch ($Action) {
        'plan' { & $terraformExe "-chdir=$terraformDir" plan '-input=false' '-out=deploy.tfplan' }
        'apply' {
            if (-not (Test-Path -LiteralPath (Join-Path $terraformDir 'deploy.tfplan'))) {
                throw 'Create and review a saved plan first.'
            }
            & $terraformExe "-chdir=$terraformDir" apply 'deploy.tfplan'
        }
        default { & $terraformExe "-chdir=$terraformDir" $Action }
    }
    if ($LASTEXITCODE -ne 0) { throw "Terraform $Action failed." }
} finally {
    $env:YC_TOKEN = $previousToken
    $env:TF_CLI_CONFIG_FILE = $previousConfig
    Remove-Variable issuedToken -ErrorAction SilentlyContinue
}
