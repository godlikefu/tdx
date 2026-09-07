// pm2 启动配置 — tdx HTTP API 服务 (cmd/tdx-api)
//
// 用法:
//   go build -o output/pm2/bin/tdx-api ./cmd/tdx-api   # 1. 编译(产物在 output/ 下, 已 gitignore)
//   pm2 start ecosystem.config.js                      # 2. 启动并守护
//   pm2 logs tdx-api                                   # 3. 日志(也可直接看 output/pm2/logs/)
//   pm2 restart tdx-api   /  pm2 stop tdx-api  /  pm2 delete tdx-api
//   pm2 save && pm2 startup                            # 可选: 开机自启
//
// 环境变量(可选, 均有默认值, 对应 cmd/tdx-api 的启动 flags):
//   TDX_ADDR=:8080  TDX_POOL=1  TDX_MAC_POOL=1  TDX_MAC=true  TDX_EX=false  TDX_DEBUG=false
//   例: TDX_ADDR=:9090 TDX_POOL=4 TDX_EX=true pm2 start ecosystem.config.js
//   已启动过需带环境变量重启时: pm2 delete tdx-api 后重新 start, 或 pm2 restart tdx-api --update-env
//
// 说明: 编译产物/日志统一放 ./output/pm2/ 下(output/ 已在 .gitignore, 构建产物不入库)。
const path = require('path');
const fs = require('fs');

const BIN = path.join(__dirname, 'output', 'pm2', 'bin', 'tdx-api');

if (!fs.existsSync(BIN)) {
  throw new Error(
    `未找到可执行文件: ${BIN}\n` +
    `请先编译: go build -o output/pm2/bin/tdx-api ./cmd/tdx-api`
  );
}

module.exports = {
  apps: [
    {
      name: 'tdx-api',
      script: BIN,
      cwd: __dirname, // sqlite 缓存等相对路径资源(./data/)统一落在项目根目录
      args: [
        '-addr', process.env.TDX_ADDR || ':8080',
        '-pool', process.env.TDX_POOL || '1',
        '-mac-pool', process.env.TDX_MAC_POOL || '1',
        ...(process.env.TDX_MAC === 'false' ? ['-mac=false'] : []), // 默认启用 /mac/* 路由
        ...(process.env.TDX_EX === 'true' ? ['-ex'] : []),          // 默认不启用 /ex/* 路由
        ...(process.env.TDX_DEBUG === 'true' ? ['-debug'] : []),
      ],
      autorestart: true,
      restart_delay: 3000,   // 行情服务器偶发不可达时避免疯狂重启, 3s 后再拉起
      max_restarts: 50,
      kill_timeout: 5000,    // 优雅退出宽限期
      time: true,            // 日志带时间戳
      out_file: path.join(__dirname, 'output', 'pm2', 'logs', 'tdx-api-out.log'),
      error_file: path.join(__dirname, 'output', 'pm2', 'logs', 'tdx-api-error.log'),
      merge_logs: true,
    },
  ],
};
