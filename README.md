# Voxlattice

基于 Gemini Live 的轻量 TTS 服务，提供简单的 HTTP 接口，把文本直接转成 WAV 音频。

**功能**
- `/tts` 文本转语音，返回 `audio/wav`
- `/voices` 获取可用音色列表（已排序）
- `/health` 健康检查与模型信息
- 内置 CORS 允许浏览器直接调用
- WAV 输出为 24kHz / 16-bit / 单声道

**运行环境**
- Go `1.24.4`（见 `go.mod`）
- 需要有效的 `GEMINI_API_KEY`

**快速开始**
1. 准备环境变量（推荐 `.env`）
2. 启动服务
3. 调用接口生成音频

```bash
# 启动
go run .
```

**运行参数**
- `--log` 指定日志输出文件路径（不带时输出到控制台）
- `--log-level` 日志级别：`debug` / `info` / `warn` / `error`（默认 `warn`）
- `--env` 指定 `.env` 路径（默认 `--config/.env`，或读取 `AUDIOMESH_ENV`）
- `--config` 指定配置目录（`Voices.json` 读取/写入位置，默认当前目录）
- `--install` 安装为系统服务
- `--uninstall` 卸载系统服务
- `--service-name` 指定服务名（默认 `voxlattice`）
说明：
- 程序会在 `--config` 指定目录读取或写入 `Voices.json`
- `.env` 默认也跟随 `--config` 目录（除非显式设置 `--env` 或 `AUDIOMESH_ENV`）
- 若 `Voices.json` 存在且内容合规，则直接使用
- 若 `Voices.json` 不存在或不合规，则使用内置默认音色并生成文件

`Voices.json` 示例（自动生成，可手动修改）：
```json
{
  "generated_at": "2026-02-07T10:00:00Z",
  "voices": [
    { "name": "kore", "description": "Kore - Female voice" },
    { "name": "charon", "description": "Charon - Male voice" }
  ]
}
```

日志说明：
- 单文件最大 10MB，超过自动覆盖（从头写入）
- Windows 路径包含空格时请用引号包住参数
- `--log` 必须带路径；不传 `--log` 才是输出控制台

**环境变量**
```dotenv
GEMINI_API_KEY=你的_key
GEMINI_MODEL=models/gemini-2.5-flash-native-audio-preview-12-2025
AUDIOMESH_PORT=8080
```

说明：
- `GEMINI_API_KEY` 可选（未提供时需在请求里传 Key）
- `GEMINI_MODEL` 选填，未设置时使用默认模型
- `AUDIOMESH_PORT` 监听端口（默认 8080）
- 程序会读取 `.env`，并在缺少键时写入默认占位值

**API**

`POST /tts`  
请求体：
```json
{
  "text": "Hello from Voxlattice",
  "voice": "kore",
  "lang": "en-US"
}
```

字段说明：
- `text` 必填
- `voice` 选填，需在 `/voices` 列表中
- `lang` 选填，例如 `en-US`、`zh-CN`

鉴权说明：
- 可以在请求头传 Key：`X-Gemini-Api-Key` 或 `X-API-Key`
- 也支持 `Authorization: Bearer <key>`
- 若请求未携带 Key，则使用 `.env` 中的 `GEMINI_API_KEY`

返回：
- `Content-Type: audio/wav`
- 二进制 WAV

示例（PowerShell）：
```powershell
$body = @{ text = "Hello from Voxlattice"; voice = "kore"; lang = "en-US" } | ConvertTo-Json
Invoke-WebRequest -Uri "http://localhost:8080/tts" -Method Post -ContentType "application/json" -Body $body -OutFile "out.wav"
```

`GET /voices`  
返回示例：
```json
[
  { "name": "amber", "description": "Amber - Female voice" },
  { "name": "charon", "description": "Charon - Male voice" }
]
```

`GET /health`  
返回示例：
```json
{
  "status": "healthy",
  "model": "models/gemini-2.5-flash-native-audio-preview-12-2025",
  "voices": { "charon": "Charon - Male voice" },
  "message": "Voxlattice TTS service ready with custom voice support"
}
```

**常见问题**
- `unsupported voice`：`voice` 不在 `/voices` 列表里
- `missing GEMINI_API_KEY`：未正确设置 API Key
- `read failed` / `live connect failed`：上游连接问题，可重试

**开发与测试**
```bash
go test ./...
```

**安装为系统服务**

内置 `--install` 选项可直接安装为系统服务（需管理员/Root 权限）。

Linux（systemd）：
```bash
go build -o /usr/local/bin/voxlattice
sudo /usr/local/bin/voxlattice --install --log /var/log/voxlattice.log --log-level warn
```

Windows（服务）：
```powershell
go build -o C:\Voxlattice\Voxlattice.exe
Start-Process -FilePath "C:\Voxlattice\Voxlattice.exe" -ArgumentList "--install --log `"C:\Program Files\Voxlattice\logs\app.log`" --log-level warn" -Verb RunAs
```

默认行为：
- Linux：写入 `/etc/voxlattice.env` 和 `/etc/systemd/system/voxlattice.service`
- Windows：写入 `C:\Voxlattice\.env`，并使用 `sc.exe` 创建服务

可选参数：
- `--service-name` 指定服务名（默认 `voxlattice`）
- `--env` 指定 `.env` 路径
- 也可通过环境变量 `AUDIOMESH_ENV` 指定 `.env` 路径

说明：
- 安装后请编辑生成的 `.env`，填入真实的 `GEMINI_API_KEY`
- Linux 会自动执行 `systemctl enable --now`
- Windows 会调用 `setx /M` 写入 `AUDIOMESH_ENV`
- 请不要用 `go run . --install`，必须使用编译后的可执行文件

**卸载系统服务**
```bash
/usr/local/bin/voxlattice --uninstall
```
```powershell
Start-Process -FilePath "C:\Voxlattice\Voxlattice.exe" -ArgumentList "--uninstall" -Verb RunAs
```
