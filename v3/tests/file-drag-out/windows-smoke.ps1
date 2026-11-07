param([string]$Binary, [string]$Receiver)
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
[Windows.Forms.Application]::SetHighDpiMode([Windows.Forms.HighDpiMode]::PerMonitorV2) | Out-Null

if ($Receiver) {
    $form = New-Object System.Windows.Forms.Form
    $form.Text = 'Wails file drag receiver'
    $form.Size = New-Object System.Drawing.Size(300, 220)
    $form.AllowDrop = $true
    $form.add_Shown({ [IO.File]::WriteAllText($Receiver + '.hwnd', $form.Handle.ToInt64().ToString()) })
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
    public struct Point { public int X; public int Y; }
    [DllImport("user32.dll")] public static extern bool ClientToScreen(IntPtr hwnd, ref Point point);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
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

function Client-Point([IntPtr]$Window, [int]$X, [int]$Y) {
    $point = [DragInput+Point]::new()
    $point.X = $X; $point.Y = $Y
    if (![DragInput]::ClientToScreen($Window, [ref]$point)) { throw 'Could not map native client coordinates.' }
    return $point
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('wails-file-drag-' + [Guid]::NewGuid())
New-Item -ItemType Directory -Path (Join-Path $root 'another folder') -Force | Out-Null
$files = @((Join-Path $root 'sample #1 ü.txt'), (Join-Path $root 'another folder/second.txt'))
$files | ForEach-Object { [IO.File]::WriteAllText($_, 'native file drag fixture') }
$output = Join-Path $root 'received.json'
$stdout = Join-Path $root 'source.log'
$stderr = Join-Path $root 'source.err'
function Read-SharedFile([string]$Path) {
    if (!(Test-Path -LiteralPath $Path)) { return '' }
    $stream = [IO.File]::Open($Path, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
    $reader = [IO.StreamReader]::new($stream)
    try { return $reader.ReadToEnd() } finally { $reader.Dispose() }
}
function Read-SourceLog { return ((Read-SharedFile $stdout) + (Read-SharedFile $stderr)) }

$target = $null
$source = $null
try {
    $target = Start-Process pwsh -PassThru -ArgumentList @('-STA', '-NoProfile', '-File', "`"$PSCommandPath`"", '-Receiver', "`"$output`"") -RedirectStandardError (Join-Path $root 'receiver.err')
    $arguments = $files | ForEach-Object { "`"$_`"" }
    $source = Start-Process (Resolve-Path $Binary) -PassThru -ArgumentList $arguments -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $src = Wait-For { $match = [regex]::Match((Read-SourceLog), 'file-drag-out native: (\d+)'); if ($match.Success) { [IntPtr]::new([long]$match.Groups[1].Value) } } 'source window'
    $dst = Wait-For { $text = Read-SharedFile ($output + '.hwnd'); if ($text -match '^\d+$') { [IntPtr]::new([long]$text) } } 'receiver window'
    Wait-For { (Read-SourceLog).Contains('file-drag-out ready') } 'frontend runtime' | Out-Null
    [DragInput]::SetWindowPos($src, [IntPtr]::Zero, 30, 30, 300, 220, 0x40) | Out-Null
    [DragInput]::SetWindowPos($dst, [IntPtr]::Zero, 450, 30, 300, 220, 0x40) | Out-Null
    function Invoke-Drag([bool]$Cancel) {
        $starts = [regex]::Matches((Read-SourceLog), 'Starting native file drag-out').Count
        [DragInput]::SetWindowPos($src, [IntPtr]::Zero, 30, 30, 300, 220, 0x40) | Out-Null
        [DragInput]::SetForegroundWindow($src) | Out-Null
        Wait-For { [DragInput]::GetForegroundWindow() -eq $src } 'source focus' | Out-Null
        $origin = Client-Point $src 120 100
        [DragInput]::SetCursorPos(350, 300) | Out-Null
        [DragInput]::SetCursorPos($origin.X, $origin.Y) | Out-Null
        Wait-For { [regex]::Matches((Read-SourceLog), 'file-drag-out trace: armed').Count -gt [regex]::Matches((Read-SourceLog), 'file-drag-out ended:').Count } 'armed source' | Out-Null
        [DragInput]::mouse_event(2, 0, 0, 0, [UIntPtr]::Zero)
        Start-Sleep -Milliseconds 60
        foreach ($offset in @(30, 40, 50)) { [DragInput]::SetCursorPos(($origin.X + $offset), $origin.Y) | Out-Null; Start-Sleep -Milliseconds 50 }
        Wait-For { [regex]::Matches((Read-SourceLog), 'Starting native file drag-out').Count -gt $starts } 'native drag start' | Out-Null
        if ($Cancel) {
            [DragInput]::keybd_event(0x1b, 0, 0, [UIntPtr]::Zero)
            [DragInput]::keybd_event(0x1b, 0, 2, [UIntPtr]::Zero)
        } else {
            $drop = Client-Point $dst 150 100
            foreach ($step in 1..10) {
                $x = [int]($origin.X + ($drop.X - $origin.X) * $step / 10)
                $y = [int]($origin.Y + ($drop.Y - $origin.Y) * $step / 10)
                [DragInput]::SetCursorPos($x, $y) | Out-Null
                Start-Sleep -Milliseconds 50
            }
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
    if (Test-Path (Join-Path $root 'receiver.err')) { Get-Content (Join-Path $root 'receiver.err') }
    throw
} finally {
    [DragInput]::mouse_event(4, 0, 0, 0, [UIntPtr]::Zero)
    foreach ($process in @($source, $target)) { if ($process -and !$process.HasExited) { Stop-Process -Id $process.Id -Force } }
    Remove-Item $root -Recurse -Force
}
