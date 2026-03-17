# 将 Cursor 用户数据从 C 盘迁移到 E 盘，并使用新数据目录启动
# 用法：以管理员或当前用户运行均可；首次运行会复制数据并创建快捷方式

$CursorDataC = "$env:APPDATA\Cursor"
$CursorDataE = "E:\DevData\Cursor"
$WshShell = New-Object -ComObject WScript.Shell

# 1. 确保 E 盘目录存在
if (-not (Test-Path "E:\DevData")) {
    New-Item -ItemType Directory -Path "E:\DevData" -Force
}
if (-not (Test-Path $CursorDataE)) {
    New-Item -ItemType Directory -Path $CursorDataE -Force
}

# 2. 若 E 盘尚无完整数据，从 C 盘复制（保留 C 盘原数据不删除）
if (-not (Test-Path "$CursorDataE\User\settings.json") -and (Test-Path $CursorDataC)) {
    Write-Host "正在将 Cursor 数据从 C 盘复制到 E 盘，请稍候..."
    robocopy $CursorDataC $CursorDataE /E /COPY:DAT /R:2 /W:3 /MT:4 /NFL /NDL /NJH /NJS
    if ($LASTEXITCODE -ge 8) {
        Write-Warning "复制可能有问题，请检查。Robocopy 退出码: $LASTEXITCODE"
    } else {
        Write-Host "复制完成。"
    }
} elseif (Test-Path $CursorDataE) {
    Write-Host "E 盘 Cursor 数据目录已存在，跳过复制。"
} else {
    Write-Warning "未找到 C 盘 Cursor 数据: $CursorDataC，仅创建 E 盘目录。"
}

# 3. 查找 Cursor 可执行文件（常见位置）
$cursorExe = $null
foreach ($path in @(
    "D:\cursor\Cursor.exe",
    "$env:LOCALAPPDATA\Programs\cursor\Cursor.exe",
    "${env:ProgramFiles}\Cursor\Cursor.exe",
    "${env:ProgramFiles(x86)}\Cursor\Cursor.exe"
)) {
    if (Test-Path $path) {
        $cursorExe = $path
        break
    }
}
if (-not $cursorExe) {
    $cursorExe = (Get-Command cursor -ErrorAction SilentlyContinue).Source
    if ($cursorExe) {
        $cursorExe = (Get-Item $cursorExe).Directory.Parent.FullName + "\Cursor.exe"
        if (-not (Test-Path $cursorExe)) { $cursorExe = $null }
    }
}
if (-not $cursorExe -or -not (Test-Path $cursorExe)) {
    Write-Warning "未找到 Cursor.exe，请手动创建快捷方式，目标为："
    Write-Host "  `"<Cursor安装路径>\Cursor.exe`" --user-data-dir=`"$CursorDataE`""
    exit 1
}

# 4. 在 E 盘数据目录下创建“使用 E 盘数据”的快捷方式
$lnkPath = "$CursorDataE\Cursor - 使用E盘数据.lnk"
$lnk = $WshShell.CreateShortcut($lnkPath)
$lnk.TargetPath = $cursorExe
$lnk.Arguments = "--user-data-dir=`"$CursorDataE`""
$lnk.WorkingDirectory = [System.IO.Path]::GetDirectoryName($cursorExe)
$lnk.Save()
Write-Host "已创建快捷方式: $lnkPath"
Write-Host "以后请用该快捷方式启动 Cursor，数据将保存在 E 盘。"
Write-Host "（原 C 盘数据未删除，可备份后自行清理以释放空间。）"
