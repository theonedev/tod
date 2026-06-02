#requires -Version 5.1
$ErrorActionPreference = 'Stop'

$BaseUrl = 'https://code.onedev.io/onedev/tod/~site'

function Get-PlatformArch {
	$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
		'ARM64' { 'arm64' }
		'AMD64' { 'amd64' }
		default {
			switch ($env:PROCESSOR_ARCHITEW6432) {
				'ARM64' { 'arm64' }
				'AMD64' { 'amd64' }
				default {
					throw "Unsupported architecture: $($env:PROCESSOR_ARCHITECTURE)"
				}
			}
		}
	}
	return "windows-$arch"
}

function Get-InstallDir {
	$existing = Get-Command tod -ErrorAction SilentlyContinue
	if (-not $existing) {
		$existing = Get-Command tod.exe -ErrorAction SilentlyContinue
	}
	if ($existing) {
		return Split-Path -Parent $existing.Source
	}

	if ($env:INSTALL_DIR) {
		return $env:INSTALL_DIR
	}

	$localBin = Join-Path $env:USERPROFILE '.local\bin'
	$pathEntries = $env:PATH -split ';'
	if ($pathEntries -contains $localBin) {
		return $localBin
	}

	return $localBin
}

function Install-Tod {
	$platformArch = Get-PlatformArch
	$binary = 'tod.exe'
	$url = "$BaseUrl/$platformArch/$binary"
	$destDir = Get-InstallDir
	$dest = Join-Path $destDir $binary

	if (-not (Test-Path $destDir)) {
		New-Item -ItemType Directory -Path $destDir -Force | Out-Null
	}

	$tmp = Join-Path $destDir ".tod-install.$([Guid]::NewGuid().ToString('N')).tmp"
	try {
		Write-Host "Downloading tod from $url..."
		Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing
		Move-Item -Path $tmp -Destination $dest -Force
		Write-Host "Installed tod to $dest"
	}
	finally {
		if (Test-Path $tmp) {
			Remove-Item $tmp -Force -ErrorAction SilentlyContinue
		}
	}

	if (-not (Get-Command tod -ErrorAction SilentlyContinue)) {
		Write-Host ''
		Write-Host "Add $destDir to your PATH, for example:"
		Write-Host "  [Environment]::SetEnvironmentVariable('PATH', `$env:PATH + ';$destDir', 'User')"
		Write-Host 'Then restart your terminal.'
	}
}

Install-Tod
