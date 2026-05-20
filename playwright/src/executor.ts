import { chromium, Browser, Page } from 'playwright';
import { ExecuteRequest, ExecuteResponse, PageContext } from './types';

const DEFAULT_TIMEOUT = 30000;
const DEFAULT_CONCURRENT = 5;

export async function executeScript(req: ExecuteRequest): Promise<ExecuteResponse> {
  const start = Date.now();
  const timeout = req.timeout || DEFAULT_TIMEOUT;
  const logs: string[] = [];

  let browser: Browser | null = null;

  try {
    // 启动浏览器
    const launchOptions: Parameters<typeof chromium.launch>[0] = {
      headless: true,
    };

    if (req.proxy) {
      launchOptions.proxy = {
        server: req.proxy.server,
        username: req.proxy.username,
        password: req.proxy.password,
      };
    }

    browser = await chromium.launch(launchOptions);

    const context = await browser.newContext({
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/135.0.0.0 Safari/537.36',
    });

    const page = await context.newPage();

    // 捕获 console.log
    page.on('console', (msg) => {
      const text = msg.text();
      logs.push(text);
    });

    // 也捕获 pageerror
    page.on('pageerror', (err) => {
      logs.push(`[pageerror] ${err.message}`);
    });

    // 编译并执行用户脚本
    const scriptFn = compileScript(req.script);
    const pageContext: PageContext = {
      proxy: req.proxy,
      url: req.url,
    };

    // 带超时执行
    const result = await Promise.race([
      scriptFn(page, pageContext),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error(`Script execution timed out after ${timeout}ms`)), timeout)
      ),
    ]);

    const finalUrl = page.url();
    const title = await page.title().catch(() => undefined);

    let screenshot: string | undefined;
    if (req.screenshot) {
      const screenshotBuffer = await page.screenshot({ type: 'png', fullPage: true });
      screenshot = screenshotBuffer.toString('base64');
    }

    await browser.close();
    browser = null;

    return {
      ok: true,
      result: Boolean(result),
      final_url: finalUrl,
      title,
      logs,
      screenshot,
      duration_ms: Date.now() - start,
    };
  } catch (error) {
    if (browser) {
      await browser.close().catch(() => {});
    }

    return {
      ok: false,
      result: false,
      logs,
      error: error instanceof Error ? error.message : String(error),
      duration_ms: Date.now() - start,
    };
  }
}

function compileScript(script: string): (page: Page, context: PageContext) => Promise<unknown> {
  // 包装用户脚本，提取 check 函数
  const wrapped = `
    ${script}
    if (typeof check !== 'function') {
      throw new Error('Script must define an async function named "check"');
    }
    check;
  `;

  // 使用 Function 构造器创建函数（在 Node.js 上下文中）
  // 注意：这里我们返回一个函数，该函数接收 page 和 context
  const fn = new Function('page', 'context', `
    return (async () => {
      ${script}
      if (typeof check !== 'function') {
        throw new Error('Script must define an async function named "check"');
      }
      return await check(page, context);
    })();
  `);

  return fn as (page: Page, context: PageContext) => Promise<unknown>;
}
