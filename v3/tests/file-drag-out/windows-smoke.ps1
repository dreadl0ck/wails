param([string]$Binary, [string]$Receiver)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms

if ($Receiver) {
    $form = New-Object System.Windows.Forms.Form
    $form.Text = 'Wails file drag receiver'
    $form.Size = New-Object System.Drawing.Size(300, 220)
    $form.AllowDrop = $true
    $form.add_DragEnter({ if ($_.Data.GetDataPresent([Windows.Forms.DataFormats]::FileDrop)) { $_.Effect = [Windows.Forms.DragDropEffects]::Copy } })
    $form.add_DragDrop({
        $files = [string[]]$_.Data.GetData([Windows.Forms.DataFormats]::FileDrop)
        [IO.File]::WriteAllText($Receiver, (ConvertTo-Json -InputObject $files -Compress))
    })
    [Windows.Forms.Application]::Run($form)
    exit
}
if (!$Binary) { throw 'Pass the file-drag-out probe binary.' }

Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class DragInput {
    [DllImport("user32.dll", CharSet=CharSet.Unicode)] public static extern IntPtr FindWindow(string cls, string title);
    [DllImport("user32.dll")] public static extern bool SetWindowPos(IntPtr hwnd, IntPtr after, int x, int y, int width, int height, uint flags);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hwnd);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int x, int y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint flags, uint dx, uint dy, uint data, UIntPtr extra);
    [DllImport("user32.dll")] public static extern void keybd_event(byte key, byte scan, uint flags, UIntPtr extra);
}
'@

function Wait-For($Check, [string]$Label) {
    $until = [DateTime]::UtcNow.AddSeconds(20)
    while ([DateTime]::UtcNow -lt $until) {
        $value = & $Check
        if ($value) { return $value }
        Start-Sleep -Milliseconds 50
    }
    throw "Timed out waiting for $Label"
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('wails-file-drag-' + [Guid]::NewGuid())
New-Item -ItemType Directory -Path (Join-Path $root 'another folder') -Force | Out-Null
$files = @((Join-Path $root 'sample #1 ü.txt'), (Join-Path $root 'another folder/second.txt'))
$files | ForEach-Object { [IO.File]::WriteAllText($_, 'native file drag fixture') }
$output = Join-Path $root 'received.json'
$stdout = Join-Path $root 'source.log'
$stderr = Join-Path $root 'source.err'
function Read-SourceLog { return (([IO.File]::ReadAllText($stdout)) + ([IO.File]::ReadAllText($stderr))) }

$target = $null
$source = $null
try {
    $target = Start-Process pwsh -PassThru -ArgumentList @('-NoProfile', '-File', "`"$PSCommandPath`"", '-Receiver', "`"$output`"")
    $arguments = $files | ForEach-Object { "`"$_`"" }
    $source = Start-Process (Resolve-Path $Binary) -PassThru -ArgumentList $arguments -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $src = Wait-For { $h = [DragInput]::FindWindow($null, 'Wails file drag source'); if ($h -ne [IntPtr]::Zero) { $h } } 'source window'
    $dst = Wait-For { $h = [DragInput]::FindWindow($null, 'Wails file drag receiver'); if ($h -ne [IntPtr]::Zero) { $h } } 'receiver window'
    Wait-For { (Read-SourceLog).Contains('file-drag-out ready') } 'frontend runtime' | Out-Null
    [DragInput]::SetWindowPos($src, [IntPtr]::Zero, 30, 30, 300, 220, 0x40) | Out-Null
    [DragInput]::SetWindowPos($dst, [IntPtr]::Zero, 450, 30, 300, 220, 0x40) | Out-Null
    function Invoke-Drag([bool]$Cancel) {
        $starts = [regex]::Matches((Read-SourceLog), 'Starting native file drag-out').Count
        [DragInput]::SetForegroundWindow($src) | Out-Null
        [DragInput]::SetCursorPos(350, 300) | Out-Null
        [DragInput]::SetCursorPos(150, 150) | Out-Null
        Wait-For { [regex]::Matches((Read-SourceLog), 'file-drag-out trace: armed').Count -gt [regex]::Matches((Read-SourceLog), 'file-drag-out ended:').Count } 'armed source' | Out-Null
        [DragInput]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)
        Start-Sleep -Milliseconds 60
        foreach ($x in @(180, 190, 200)) { [DragInput]::SetCursorPos($x, 150) | Out-Null; Start-Sleep -Milliseconds 50 }
        Wait-For { [regex]::Matches((Read-SourceLog), 'Starting native file drag-out').Count -gt $starts } 'native drag start' | Out-Null
        if ($Cancel) {
            [DragInput]::keybd_event(0x1b, 0, 0, [UIntPtr]::Zero)
            [DragInput]::keybd_event(0x1b, 0, 2, [UIntPtr]::Zero)
        } else {
            foreach ($x in (220..600 | Where-Object { $_ % 40 -eq 0 })) { [DragInput]::SetCursorPos($x, 150) | Out-Null; Start-Sleep -Milliseconds 50 }
        }
        [DragInput]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
    }
    Invoke-Drag $false
    Wait-For { Test-Path $output } 'native file transfer' | Out-Null
    $received = Get-Content $output -Raw | ConvertFrom-Json
    if (($received -join '|') -cne ($files -join '|')) { throw 'Native file paths changed during transfer.' }
    Wait-For { (Read-SourceLog).Contains('performed:true') } 'successful completion' | Out-Null
    Remove-Item $output
    Invoke-Drag $true
    Wait-For { (Read-SourceLog).Contains('performed:false') } 'cancellation callback' | Out-Null
    if (Test-Path $output) { throw 'Cancelled drag delivered files.' }
    Invoke-Drag $false
    Wait-For { Test-Path $output } 'drag after cancellation' | Out-Null
    $received = Get-Content $output -Raw | ConvertFrom-Json
    if (($received -join '|') -cne ($files -join '|')) { throw 'Second transfer changed the file paths.' }
    Write-Output 'PASS: native file transfer, Unicode/spaces, cancellation, and reuse'
} catch {
    if (Test-Path $stdout) { Get-Content $stdout }
    if (Test-Path $stderr) { Get-Content $stderr }
    throw
} finally {
    [DragInput]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
    foreach ($process in @($source, $target)) { if ($process -and !$process.HasExited) { Stop-Process -Id $process.Id -Force } }
    Remove-Item $root -Recurse -Force
}
