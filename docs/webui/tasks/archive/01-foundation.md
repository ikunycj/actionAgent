# 01. Foundation

## 目标
完成 WebUI 工程骨架，建立后续所有页面和功能的统一基础设施。

## 前置依赖
1. 已确认技术栈：React、TypeScript、Vite、Tailwind CSS、Redux Toolkit、RTK Query。
2. 已确认目录与模块边界：见 `docs/webui/ARCHITECTURE.md`。

## 交付物
1. `web/` 前端工程初始化完成。
2. 路由、状态管理、样式系统、基础 UI 层、API 适配层骨架完成。
3. 本地开发、构建、类型检查、基础测试命令可运行。

## 任务清单
- [x] 1.1 初始化 Vite React TypeScript 工程。
- [x] 1.2 接入 Tailwind CSS、PostCSS、基础主题变量和全局样式。
- [x] 1.3 接入 Redux Toolkit、RTK Query、React Router。
- [x] 1.4 建立 `src/app`、`src/shared`、`src/features`、`src/widgets`、`src/pages` 目录骨架。
- [x] 1.5 配置 `tsconfig paths` 与 Vite alias。
- [x] 1.6 建立 `coreHttpApi` 与 `coreBridgeApi` 的最小空壳。
- [x] 1.7 建立 `AppShell`、`Button`、`Input`、`StatusBadge` 等基础组件占位实现。
- [x] 1.8 建立统一错误对象、环境变量读取与本地存储工具。
- [x] 1.9 接入 ESLint、Vitest、React Testing Library、Playwright 基础配置。
- [x] 1.10 确认 `dev/build/test/typecheck` 脚本命名。

## 验收标准
1. `web/` 可独立启动开发服务器。
2. 路由、Store、Tailwind、RTK Query 已完成最小集成。
3. 目录结构与 `docs/webui/ARCHITECTURE.md` 一致。
4. 后续功能开发不需要再返工基础工程组织。
