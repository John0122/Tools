#!/usr/bin/env python3
"""生成可静态提取的固定演示页面与样例清单；不在浏览器中模拟网络结果。"""

from html import escape as esc
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / "frontend"
# 所有生成页面沿用首页的内嵌图标，避免新增图标请求干扰外链检测样例。
FAVICON = '<link rel="icon" href="data:image/svg+xml,%3Csvg%20xmlns=%22http://www.w3.org/2000/svg%22%20width=%2224%22%20height=%2224%22%20viewBox=%220%200%2024%2024%22%20fill=%22none%22%20stroke=%22%230d8e91%22%20stroke-width=%222%22%20stroke-linecap=%22round%22%20stroke-linejoin=%22round%22%20class=%22icon%20icon-tabler%20icons-tabler-outline%20icon-tabler-brand-feedly%22%3E%3Cpath%20stroke=%22none%22%20d=%22M0%200h24v24H0z%22%20fill=%22none%22%20/%3E%3Cpath%20d=%22M7.833%2012.278l4.445%20-4.445%22%20/%3E%3Cpath%20d=%22M10.055%2014.5l2.223%20-2.222%22%20/%3E%3Cpath%20d=%22M12.278%2016.722l.555%20-.555%22%20/%3E%3Cpath%20d=%22M19.828%2014.828a4%204%200%200%200%200%20-5.656l-5%20-5a4%204%200%200%200%20-5.656%200l-5%205a4%204%200%200%200%200%205.656l6.171%206.172h3.314l6.171%20-6.172%22%20/%3E%3C/svg%3E">'
EXT = "https://__LAB_EXTERNAL_HOST__:38443"
# 部署时注入本站 IP，使原始 HTML 能直接被静态扫描器提取。
INTERNAL_IP = "https://__LAB_INTERNAL_HOST__:38443"
CATEGORIES = [
    ("navigation", "页面跳转外链", "超链接、动态导航与页面元信息", "P0 · P1 · P2"),
    ("javascript", "JavaScript 外链", "外部脚本、模块导入与动态资源", "P0 · P1 · P2"),
    ("css", "CSS 外链", "样式文件、CSS 导入与背景资源", "P0 · P1"),
    ("images", "图片外链", "图片地址、响应式图片与多种格式", "P0"),
    ("media", "音视频外链", "音频、视频、播放列表与外部播放器", "P1"),
    ("api", "API 外链", "Fetch、XHR 与实际 GraphQL 查询", "P0"),
    ("websocket", "WebSocket 外链", "WS / WSS 连接、消息与关闭", "P1"),
    ("iframe", "iframe 外链", "嵌入文档、地图与组件样例", "P0"),
    ("downloads", "文件下载外链", "PDF、DOCX、ZIP、EXE 下载响应", "P1"),
    ("fonts", "字体外链", "WOFF、WOFF2、TTF、OTF 字体加载", "P1"),
    ("forms", "表单提交外链", "GET / POST、编码方式与提交地址覆盖", "P0"),
    ("redirects", "重定向外链", "HTTP、Meta Refresh 与 JavaScript 跳转", "P1"),
    ("ip-address", "IP 地址链接", "内站 IP、公网外站与局域网其他设备", "P0 · P1"),
]
CASES = []


def write(path, text):
    p = ROOT / path
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(text, encoding="utf-8")


def record(category, key, feature, target, status=200, scope="external", visibility="visible", method="GET", trigger="load", ip_group=None):
    case_id = f"{category}-{key}"
    CASES.append(dict(id=case_id, category=category, page=f"/links/{category}/", feature=feature,
                      target=target, expected_status=status, scope=scope, visibility=visibility,
                      method=method, trigger=trigger))
    # 保留原有站点归属字段，IP 分组另记，避免把局域网地址误当作站内地址。
    if ip_group is not None:
        CASES[-1]["ip_group"] = ip_group
    return case_id


def link(url, label, case_id="", hidden=False):
    # 资源链接直接在 HTML 中声明新标签页，便于静态扫描并保留当前测试页。
    focus = ' tabindex="-1"' if hidden else ""
    return f'<a href="{esc(url, quote=True)}" target="_blank" rel="noopener" data-case-id="{case_id}"{focus}>{esc(label)}</a>'


def section(key, title, feature, body, note="", priority="P0"):
    return f'''<section class="lab-section" id="{key}">
      <div class="lab-section-heading"><h2>{title}</h2><span class="lab-priority">{priority}</span></div>
      <p class="lab-feature"><code>{esc(feature)}</code></p>
      <div class="lab-demo">{body}</div>{f'<p class="lab-note">{note}</p>' if note else ''}</section>'''


def request_demo(category, key, title, feature, url, kind="fetch", method="GET", body="", status=200, auto=False):
    cid = record(category,key,feature,url,status,method=method,trigger="load" if auto else "click")
    attrs = f'data-request="{esc(url, quote=True)}" data-request-kind="{kind}" data-method="{method}" data-body="{esc(body, quote=True)}"'
    if auto:
        attrs += ' data-auto="true"'
    demo = f'''<p>目标地址 <code>{esc(url)}</code></p>
      <button class="lab-button" type="button" {attrs} data-case-id="{cid}" aria-controls="result-{cid}" disabled>发送请求</button>
      <pre id="result-{cid}" class="lab-result" data-result role="status" aria-live="polite">等待发送请求。</pre>'''
    return section(cid,title,feature,demo,"响应状态和正文来自实际网络请求；失败后可以再次发送。")


def fixture_page(title, body, head=""):
    return f'''<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{title}</title>{FAVICON}{head}<style>body{{font:16px/1.7 system-ui;margin:32px;max-width:760px;color:#183044;background:#f1f5f8}}h1{{font-size:28px}}code{{overflow-wrap:anywhere}}</style></head><body><h1>{title}</h1>{body}</body></html>'''


def shared_cases(slug, index):
    inside=f"/fixtures/hidden/{slug}-internal.html"
    outside=f"{EXT}/fixtures/hidden/{slug}-external.html"
    for scope in ("internal","external"):
        write(f"fixtures/hidden/{slug}-{scope}.html",fixture_page("隐藏链接目标",f"<p>样例编号：{slug}-hidden-{scope}。该目标真实存在，预期 HTTP 200。</p>"))
    a=record(slug,"hidden-internal","a[href] + CSS hidden",inside,scope="internal",visibility="hidden",trigger="inspect")
    b=record(slug,"hidden-external","a[href] + CSS hidden",outside,visibility="hidden",trigger="inspect")
    style=("lab-hidden-display","lab-hidden-opacity","lab-hidden-offscreen")[index%3]
    # 隐藏样例显式退出键盘序列，避免正常使用者落入不可见焦点。
    hidden=f'''<div class="{style}" aria-hidden="true">{link(inside,"站内补充资料",a,hidden=True)}{link(outside,"站外补充资料",b,hidden=True)}</div>'''
    dead_in=f"/fixtures/missing/{slug}.html"
    dead_out=f"{EXT}/lab/status/410?case={slug}-dead-external"
    c=record(slug,"dead-internal","a[href]",dead_in,404,scope="internal",trigger="click")
    d=record(slug,"dead-external","a[href]",dead_out,410,trigger="click")
    record(slug,"disclosure","details / summary / a[href]","/fixtures/landing.html?case="+slug+"-disclosure",scope="internal",visibility="collapsed",trigger="expand-and-click")
    return section("visibility","暗链与正常折叠内容","a[href] · display / opacity / position",hidden+f'''<p>本区包含两条真实隐藏链接：<code>{a}</code>、<code>{b}</code>。可通过 DOM 与计算样式检查。</p>
      <details><summary>正常折叠内容对照</summary><p>{link('/fixtures/landing.html?case='+slug+'-disclosure','展开后可见的站内资料')}</p></details>''',"隐藏样例是检测数据；正常折叠、无障碍内容与恶意暗链需要分别判断。", "P1")+section("dead-links","死链响应对照","HTTP 404 · HTTP 410",f'<ul><li>{link(dead_in,"站内不存在的页面 · 预期 404",c)}</li><li>{link(dead_out,"站外已移除的页面 · 预期 410",d)}</li></ul>',"这些链接故意失效；它们的错误响应是样例的预期结果。")


def ip_address_content():
    """IP 分类单独使用字面地址；非受控目标不复用域名外站的状态承诺。"""
    slug = "ip-address"
    links = []
    for scheme, port in (("http", 38080), ("https", 38443)):
        url = f"{scheme}://__LAB_INTERNAL_HOST__:{port}/fixtures/landing.html?case=ip-address-{scheme}"
        cid = record(slug, scheme, "a[href] · IPv4", url, scope="internal", trigger="click", ip_group="internal")
        links.append(f'<li>{link(url, f"通过 {scheme.upper()} 访问内站", cid)}<p><code>{esc(url)}</code></p></li>')
    body = section("internal", "内站 IP 链接", "a[href] · IPv4 · HTTP / HTTPS", '<ul>'+''.join(links)+'</ul>',
                   "当前站点的 IP 优先归为内站，即使它本身使用局域网私有地址。HTTPS 使用本站证书。")
    for group, title, token, note in (
        ("public", "外站 IP 链接（公网）", "PUBLIC", "目标为其他站点的公网 IPv4；可达性、证书和响应状态取决于配置的目标服务。"),
        ("lan", "局域网 IP 链接", "LAN", "目标为其他设备的私有 IPv4，覆盖 10/8、172.16/12、192.168/16 网段；实际访问需具备对应网络连通性。"),
    ):
        # 未配置时，启动脚本把清单中的目标替换为 null，把样例替换为说明文字。
        record(slug, group, "a[href] · IPv4", f"__LAB_{token}_IP_URL__", status=None,
               trigger="click", ip_group=group)
        body += section(group, title, "a[href] · IPv4", f"__LAB_{token}_IP_SAMPLE__", note)

    hidden_path = "fixtures/hidden/ip-address-internal.html"
    write(hidden_path, fixture_page("内站 IP 隐藏链接目标", "<p>样例编号：ip-address-hidden-internal。该目标真实存在，预期 HTTP 200。</p>"))
    hidden_url = f"{INTERNAL_IP}/{hidden_path}"
    cid = record(slug, "hidden-internal", "a[href] · IPv4 + CSS hidden", hidden_url,
                 scope="internal", visibility="hidden", trigger="inspect", ip_group="internal")
    hidden = f'<div class="lab-hidden-display" aria-hidden="true">{link(hidden_url, "内站 IP 补充资料", cid, hidden=True)}</div>'
    body += section("visibility", "内站 IP 暗链", "a[href] · IPv4 · display: none", hidden+f'<p>本区包含真实隐藏链接 <code>{cid}</code>，可通过 DOM 与计算样式检查。</p>',
                    "隐藏链接退出键盘焦点序列，目标地址仍保留在原始 HTML 中。", "P1")
    dead_links = []
    for key, path, status in (("missing", "/fixtures/missing/ip-address.html", 404),
                              ("gone", "/lab/status/410?case=ip-address-gone", 410)):
        url = INTERNAL_IP + path
        cid = record(slug, key, "a[href] · IPv4", url, status, scope="internal", trigger="click", ip_group="internal")
        dead_links.append(f'<li>{link(url, f"内站 IP 失效目标 · 预期 {status}", cid)}</li>')
    body += section("dead-links", "内站 IP 死链响应", "HTTP 404 · HTTP 410", '<ul>'+''.join(dead_links)+'</ul>',
                    "这两条失效链接由本站提供固定响应；公网和局域网目标不预设成功或失败状态。")
    return body


def category_content(slug):
    body,head="",""
    if slug=="navigation":
        links=[]
        for key,label in [("friend","友情链接"),("article","外部文章"),("help","帮助导航"),("social","社交资料页")]:
            url=f"{EXT}/fixtures/landing.html?case=navigation-{key}"
            cid=record(slug,key,"a[href]",url,trigger="click")
            links.append(f"<li>{link(url,label,cid)}</li>")
        body+=section("anchors","页面与资料导航","a[href]",'<ul>'+''.join(links)+f'<li>{link("/fixtures/landing.html","站内有效页面对照")}</li></ul>')
        url=f"{EXT}/fixtures/landing.html?case=navigation-dynamic"
        cid=record(slug,"dynamic","JS createElement / href",url,trigger="click")
        body+=section("dynamic","动态生成导航","document.createElement('a') · href",f'<button class="lab-button" type="button" data-create-link="{url}" data-case-id="{cid}" disabled>生成动态链接</button><div class="lab-dynamic-result" data-dynamic-result role="status">点击后在此插入链接。</div>',priority="P1")
        for rel in ["canonical","alternate"]:
            url=f"{EXT}/fixtures/landing.html?case=navigation-{rel}"
            record(slug,rel,f"link[rel={rel}]",url,trigger="inspect")
            head+=f'<link rel="{rel}" href="{url}">'
        head+=f'<meta property="og:url" content="{EXT}/fixtures/landing.html?case=navigation-og"><meta property="og:image" content="{EXT}/fixtures/images/sample.png"><meta property="og:title" content="Target Web 链接演示"><meta name="twitter:card" content="summary"><meta name="twitter:image" content="{EXT}/fixtures/images/sample.png">'
        record(slug,"og","meta[property=og:url]",f"{EXT}/fixtures/landing.html?case=navigation-og",trigger="inspect")
        record(slug,"og-image","meta[property=og:image]",f"{EXT}/fixtures/images/sample.png",trigger="inspect")
        record(slug,"twitter-image","meta[name=twitter:image]",f"{EXT}/fixtures/images/sample.png",trigger="inspect")
        body+=section("metadata","规范链接与分享元数据","canonical · alternate · Open Graph · Twitter Card","<p>本页 head 中包含真实的规范地址、替代地址与分享图片。声明地址可供静态提取；浏览器不一定主动请求这些地址。</p>",priority="P2")
    elif slug=="javascript":
        for name,feature in [("demo-sdk.js","script[src]"),("demo-module.js","script[type=module][src]")]:
            url=f"{EXT}/fixtures/{name}";record(slug,name,feature,url)
            head+=f'<script {"type=module" if "module" in name else "defer"} src="{url}"></script>'
        body+=section("scripts","外部 SDK 与模块脚本","script[src] · type=module",'<p id="sdk-state" role="status">正在加载外站演示 SDK…</p><p id="module-state" role="status">正在加载外站模块及其依赖…</p>')
        url=f"{EXT}/fixtures/dynamic.js";cid=record(slug,"dynamic-script","JS createElement('script') / src",url,trigger="click")
        body+=section("dynamic","动态加载脚本","document.createElement('script') · src",f'<button class="lab-button" type="button" data-load-script="{url}" data-case-id="{cid}" disabled>加载动态脚本</button><p data-script-result role="status">等待加载。</p>',priority="P1")
        head+=f'<link rel="preload" href="{EXT}/fixtures/demo-sdk.js" as="script"><link rel="prefetch" href="{EXT}/fixtures/dynamic.js"><link rel="preconnect" href="{EXT}"><link rel="dns-prefetch" href="//__LAB_EXTERNAL_HOST__">'
        record(slug,"module-import","JS import",f"{EXT}/fixtures/module-child.js")
        record(slug,"preload","link[rel=preload]",f"{EXT}/fixtures/demo-sdk.js",trigger="browser-hint")
        record(slug,"prefetch","link[rel=prefetch]",f"{EXT}/fixtures/dynamic.js",trigger="browser-hint")
        # DNS 与预连接只声明连接意图，不虚构 HTTP 请求方法或响应码。
        record(slug,"preconnect","link[rel=preconnect]",EXT,status=None,method=None,trigger="browser-hint")
        record(slug,"dns-prefetch","link[rel=dns-prefetch]","//__LAB_EXTERNAL_HOST__",status=None,method=None,trigger="browser-hint")
        body+=section("hints","预加载与连接提示","preload · prefetch · preconnect · dns-prefetch","<p>本页声明脚本预加载、后续脚本预取，以及外站连接与 DNS 提示。提示是否实际执行由浏览器决定。</p>",priority="P2")
        head+=f'<script defer src="{EXT}/fixtures/missing/script.js"></script>'
        record(slug,"missing-script","script[src]",f"{EXT}/fixtures/missing/script.js",404)
    elif slug=="css":
        url=f"{EXT}/fixtures/demo.css";head+=f'<link rel="stylesheet" href="{url}">';record(slug,"stylesheet","link[rel=stylesheet]",url)
        record(slug,"import","CSS @import",f"{EXT}/fixtures/imported.css")
        record(slug,"background","CSS url()",f"{EXT}/fixtures/images/sample.svg")
        body+=section("stylesheet","外部样式与嵌套资源","link[rel=stylesheet] · @import · background-image: url()",'<div class="fixture-css"><p class="fixture-imported">样式文件加载后，这里会出现虚线边框和斜体文本。</p><div class="fixture-background" aria-label="CSS 外部背景示例"></div></div>')
        head+=f'<link rel="stylesheet" href="{EXT}/fixtures/missing/style.css">'
        record(slug,"missing-style","link[rel=stylesheet]",f"{EXT}/fixtures/missing/style.css",404)
    elif slug=="images":
        demos=[]
        for name in ["sample.png","sample.jpg","sample.gif","sample.svg","cherry.webp"]:
            url=f"{EXT}/fixtures/images/{name}";cid=record(slug,name,"img[src]",url)
            demos.append(f'<figure><img src="{url}" alt="{name} 格式演示图片" width="480" height="270" data-case-id="{cid}"><figcaption>{name}</figcaption></figure>')
        body+=section("formats","五种图片格式","img[src]",'<div class="lab-image-grid">'+''.join(demos)+'</div>',"WebP 照片：Benjamin Gimmel，CC BY-SA 3.0，来源与许可证见资源说明。")
        url=f"{EXT}/fixtures/images/sample.jpg";record(slug,"srcset","img[srcset] / source[srcset]",url)
        body+=section("responsive","响应式图片","picture · source[srcset] · img[srcset]",f'<picture><source type="image/webp" srcset="{EXT}/fixtures/images/cherry.webp"><img src="{url}" srcset="{url} 480w, {EXT}/fixtures/images/cherry.webp 1024w" sizes="(max-width: 800px) 90vw, 480px" alt="响应式图片选择示例" width="480" height="270"></picture>')
        url=f"{EXT}/fixtures/missing/image.png";record(slug,"missing-image","img[src]",url,404)
        body+=section("missing-image","失效图片资源","img[src] · HTTP 404",f'<img class="lab-broken-image" src="{url}" alt="预期加载失败的外站图片" width="480" height="100">')
    elif slug=="media":
        mp4=f"{EXT}/fixtures/media/flower.mp4";mp3=f"{EXT}/fixtures/media/t-rex-roar.mp3";hls=f"{EXT}/fixtures/media/sample.m3u8"
        for key,feature,url in [("video","video / source[src]",mp4),("audio","audio[src] / source[src]",mp3),("hls","source[src] / M3U8",hls)]:record(slug,key,feature,url)
        body+=section("video","视频文件与 source","video · source[src]",f'<video controls preload="metadata" width="480" height="270" poster="{EXT}/fixtures/images/sample.svg"><source src="{mp4}" type="video/mp4">浏览器不支持视频播放。</video>',"MDN 花朵视频，演示 MP4 资源请求与分段读取。", "P1")
        body+=section("audio","音频文件","audio[src] · source[src]",f'<audio controls preload="metadata" src="{mp3}"><source src="{mp3}" type="audio/mpeg">浏览器不支持音频播放。</audio>',"MDN 音频示例，点击播放会发出声音。", "P1")
        body+=section("hls","HLS 播放列表","source[src] · .m3u8",f'<audio controls preload="none"><source src="{hls}" type="application/vnd.apple.mpegurl"></audio><p>{link(hls,"查看实际 M3U8 播放列表")}</p>',"播放列表引用本地受控外站的 MP3 音频分片；播放需要支持原生 HLS 的浏览器。", "P1")
        body+=section("player","嵌入式播放器","iframe[src]",f'<iframe src="{EXT}/fixtures/player.html" title="外站播放器" loading="lazy" class="lab-frame"></iframe>',priority="P1")
        record(slug,"poster","video[poster]",f"{EXT}/fixtures/images/sample.svg")
        record(slug,"hls-segment","M3U8 Packed Audio segment",f"{EXT}/fixtures/media/roar-hls.mp3",trigger="play")
        record(slug,"player","iframe[src]",f"{EXT}/fixtures/player.html",trigger="scroll")
    elif slug=="api":
        body+=request_demo(slug,"fetch","Fetch 查询","fetch()",f"{EXT}/lab/rest?name=Fetch&case=api-fetch",auto=True)
        body+=request_demo(slug,"xhr","XHR 提交","XMLHttpRequest",f"{EXT}/lab/rest?case=api-xhr","xhr","POST",json.dumps({"name":"XHR","message":"来自三级页面的真实提交"},ensure_ascii=False))
        body+=request_demo(slug,"graphql","GraphQL 查询与变量","POST · application/json · GraphQL",f"{EXT}/lab/graphql","fetch","POST",json.dumps({"query":"query($id:ID!){service article(id:$id){id title summary}}","variables":{"id":"intro"}}))
        body+=request_demo(slug,"missing-api","失效接口","fetch() · HTTP 410",f"{EXT}/lab/status/410?case=api-missing-api",status=410)
    elif slug=="websocket":
        for proto,port in [("ws",38080),("wss",38443)]:record(slug,proto,"new WebSocket()",f"{proto}://__LAB_EXTERNAL_HOST__:{port}/lab/ws",101,trigger="click")
        body+=section("socket","建立连接并发送消息","new WebSocket() · send() · close()",'''<div data-websocket><div class="lab-actions"><button class="lab-button" type="button" data-ws-connect="wss" disabled>连接 WSS</button><button class="lab-button" type="button" data-ws-connect="ws" disabled>连接 WS</button><button class="lab-button" type="button" data-ws-close disabled>关闭连接</button></div><label for="ws-message">演示消息</label><input id="ws-message" value="你好，外链测试站" maxlength="1000"><button class="lab-button" type="button" data-ws-send disabled>发送消息</button><pre class="lab-result" data-ws-log role="status" aria-live="polite">尚未连接。</pre></div>''',"服务器回显实际消息，支持心跳，单次连接最多 2 分钟。HTTPS 页面使用 WSS；WS 可从 HTTP 页面测试。", "P1")
    elif slug=="iframe":
        for key,title in [("embedded","外站文档"),("map","地图示意组件"),("widget","服务状态组件")]:
            url=f"{EXT}/fixtures/{key}.html";cid=record(slug,key,"iframe[src]",url)
            body+=section(key,title,"iframe[src]",f'<iframe src="{url}" title="{title}" class="lab-frame" data-case-id="{cid}"></iframe>')
        url=f"{EXT}/fixtures/missing/frame.html";record(slug,"missing-frame","iframe[src]",url,404)
        body+=section("missing-frame","失效嵌入文档","iframe[src] · HTTP 404",f'<iframe src="{url}" title="预期 404 的嵌入文档" class="lab-frame lab-frame-small"></iframe>')
    elif slug=="downloads":
        links=[]
        for ext,desc in [("pdf","单页说明文档"),("docx","可打开的文字文档"),("zip","包含 README 的压缩包"),("exe","仅打印说明的 Windows 示例程序")]:
            url=f"{EXT}/fixtures/downloads/sample.{ext}";cid=record(slug,ext,"a[href] / Content-Disposition",url,trigger="click")
            links.append(f'<li>{link(url,ext.upper()+" · "+desc,cid)}</li>')
        body+=section("files","真实文件下载","a[href] · Content-Type · Content-Disposition",'<ul>'+''.join(links)+'</ul>',"服务器返回附件下载响应；文件内容与扩展名一致。", "P1")
    elif slug=="fonts":
        head+='<link rel="stylesheet" href="/fixtures/fonts/demo-fonts.css">'
        for ext in ["woff","woff2","ttf","otf"]:
            url=f"{EXT}/fixtures/fonts/SourceCodePro-Regular.{ext}";record(slug,ext,"@font-face / src: url()",url)
            body+=section(ext,ext.upper()+" 字体加载","@font-face · url()",f'<p class="fixture-font-{ext}">Target Web / 0123456789 / Link Detection</p>',"每个分区使用独立字体族，确保对应格式产生实际请求。", "P1")
        head+=f'<link rel="preload" href="{EXT}/fixtures/fonts/SourceCodePro-Regular.woff2" as="font" type="font/woff2" crossorigin>'
        record(slug,"stylesheet","link[rel=stylesheet]","/fixtures/fonts/demo-fonts.css",scope="internal")
        record(slug,"preload","link[rel=preload][as=font]",f"{EXT}/fixtures/fonts/SourceCodePro-Regular.woff2",trigger="browser-hint")
    elif slug=="forms":
        for key,title,method,encoding in [("get","GET 查询表单","get","application/x-www-form-urlencoded"),("post","POST 文本表单","post","application/x-www-form-urlencoded"),("multipart","Multipart 文本表单","post","multipart/form-data")]:
            url=f"{EXT}/lab/form";cid=record(slug,key,"form[action]",url,method=method.upper(),trigger="submit")
            record(slug,key+"-internal","button[formaction]","/lab/form",scope="internal",method=method.upper(),trigger="submit")
            demo=f'''<form action="{url}" method="{method}" enctype="{encoding}" target="form-result-{key}" data-lab-form novalidate data-case-id="{cid}">
            <label for="name-{key}">演示姓名</label><input id="name-{key}" name="name" value="测试访客" maxlength="24" required aria-describedby="form-status-{key}">
            <label for="message-{key}">演示留言</label><input id="message-{key}" name="message" value="验证表单提交地址" maxlength="200">
            <div class="lab-actions"><button class="lab-button" type="submit">提交到外站</button><button class="lab-button" type="submit" formaction="/lab/form">改为提交到站内</button></div>
            <p id="form-status-{key}" data-form-status role="status">填写演示内容后提交，响应显示在下方。</p></form>
            <iframe name="form-result-{key}" title="{title}的实际响应" class="lab-frame lab-frame-small" data-form-frame></iframe>'''
            body+=section(key,title,"form[action] · button[formaction] · "+encoding,demo)
    elif slug=="redirects":
        links=[]
        for code in [301,302,303,307,308]:
            for scope,label in [("internal","站内"),("external","站外")]:
                url=f"/lab/redirect/{code}/{scope}/ok?case=redirects-{code}-{scope}";cid=record(slug,f"{code}-{scope}","HTTP Location",url,code,scope="internal",trigger="click")
                links.append(f'<li>{link(url,f"{code} → {label}有效页面",cid)}</li>')
        body+=section("http","HTTP 重定向","301 · 302 · 303 · 307 · 308 · Location",'<ul class="lab-two-columns">'+''.join(links)+'</ul>',"入口返回真实 3xx；请检查完整跳转链及最终地址。", "P1")
        for key,label,feature in [("meta","Meta Refresh 跳转","meta[http-equiv=refresh]"),("js","JavaScript 跳转","window.location.replace()")]:
            url=f"/fixtures/{key}-redirect.html";cid=record(slug,key,feature,url,scope="internal",trigger="click")
            record(slug,key+"-target",feature,f"{EXT}/fixtures/landing.html?case=redirects-{key}-target",trigger="redirect")
            body+=section(key,label,feature,link(url,"访问跳转样例",cid),"进入样例后跳往外站有效目标；分类页本身保持可浏览。", "P1")
        url="/lab/redirect/302/external/missing";record(slug,"dead-chain","Location → 410",url,302,scope="internal",trigger="click")
        body+=section("dead-chain","跳转后失效","302 → external → 410",link(url,"访问最终返回 410 的跳转链"),priority="P1")
    elif slug=="ip-address":
        body=ip_address_content()
    return body,head


def shell(title, intro, body, slug="", head=""):
    nav=''.join(f'<li><a href="/links/{s}/"{chr(32)+"aria-current=page" if s==slug else ""}>{t}</a></li>' for s,t,_,_ in CATEGORIES)
    crumbs='<a href="/">首页</a><span aria-hidden="true">/</span>'+(f'<a href="/links/">外链检测</a><span aria-hidden="true">/</span><span aria-current="page">{title}</span>' if slug else '<span aria-current="page">外链检测</span>')
    resource_note='<p class="lab-note">资源链接在新标签页打开；文件链接按浏览器设置下载。</p>' if slug else ""
    # IP 分类不把复用同一主机的受控域名站点标成公网外站。
    origin_note=f'<p class="lab-origin">受控外站 <code>{EXT}</code><span>主站与外站通过独立主机名区分。</span></p>'
    external_meta=f'<meta name="lab-external-base" content="{EXT}">'
    shared_script='<script src="/js/link-lab.js" defer></script>'
    if slug=="ip-address":
        origin_note='<p class="lab-origin">当前站点 IP <code>__LAB_INTERNAL_HOST__</code><span>内站优先匹配本站 IP；外站限定公网 IP；局域网使用其他设备的私有 IP。</span></p>'
        # 纯静态 IP 页不加载域名演示脚本或元数据，避免扫描到无关的域名目标。
        external_meta=shared_script=""
    return f'''<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{title} · Target Web</title>
{FAVICON}{external_meta}<link rel="stylesheet" href="/css/style.css"><link rel="stylesheet" href="/css/link-lab.css">{head}{shared_script}</head>
<body class="link-lab"><a class="skip-link" href="#lab-content">跳到测试内容</a><main>
<nav class="lab-breadcrumb" aria-label="面包屑">{crumbs}</nav>
<header class="lab-header"><div class="label">TARGET WEB / 外链检测</div><h1>{title}</h1><p class="intro">{intro}</p>{resource_note}</header>
{origin_note}
<div class="lab-layout"><aside class="lab-sidebar"><nav aria-label="测试分类"><h2>分类导航</h2><ul>{nav}</ul></nav></aside>
<div id="lab-content" class="lab-content" tabindex="-1">{body}</div></div>
<footer><p><a href="/links/">返回分类目录</a> · <a href="/">返回首页</a> · {link('/links/cases.json','样例对照清单')} · {link('/fixtures/SOURCES.md','资源来源')}</p><p>页面包含预置隐藏链接和失效资源，供外链检测与结果核对使用。</p></footer></main></body></html>'''


def generate():
    # 独立外站的落地文档是演示数据，不复制主站界面。
    write("fixtures/landing.html",fixture_page("外链示例资料站","<p>这里是正常链接的目标页面，预期 HTTP 200。</p><p>这份资料介绍网站资源、接口和跳转行为。主站与受控外站均可提供它，用于对照链接归属。</p>"))
    write("fixtures/embedded.html",fixture_page("服务说明","<p>嵌入式文档加载成功。</p><p>此页面独立返回 HTML，便于核对 iframe 的地址及响应。</p>"))
    write("fixtures/map.html",fixture_page("园区地图示意","<svg viewBox='0 0 500 150' role='img' aria-label='三个园区区域示意'><path d='M50 80H450' stroke='#005f91' stroke-width='8'/><circle cx='70' cy='80' r='25' fill='#81d5fc'/><circle cx='250' cy='80' r='25' fill='#81d5fc'/><circle cx='430' cy='80' r='25' fill='#81d5fc'/><text x='45' y='135'>入口</text><text x='225' y='135'>中心</text><text x='405' y='135'>出口</text></svg>"))
    write("fixtures/widget.html",fixture_page("服务状态组件","<p>演示组件：服务运行说明</p><ul><li>静态资源：由 Nginx 提供</li><li>接口演示：由 Go 提供</li><li>数据用途：外链发现与分类测试</li></ul>"))
    write("fixtures/player.html",fixture_page("外站视频播放器",'<video controls preload="metadata" width="480" style="max-width:100%"><source src="/fixtures/media/flower.mp4" type="video/mp4"></video>'))
    write("fixtures/meta-redirect.html",fixture_page("Meta 跳转中","<p>即将跳转到外站目标。</p>",f'<meta http-equiv="refresh" content="1;url={EXT}/fixtures/landing.html?case=redirects-meta-target">'))
    write("fixtures/js-redirect.html",fixture_page("JavaScript 跳转中","<p>即将跳转到外站目标。</p><noscript>此样例需要 JavaScript 执行跳转。</noscript>",f'<script>window.location.replace("{EXT}/fixtures/landing.html?case=redirects-js-target");</script>'))
    write("fixtures/demo-sdk.js",'"use strict";\n// 受控外站 SDK 示例，只更新样例状态。\nconst sdkState = document.getElementById("sdk-state"); if (sdkState) sdkState.textContent = "外站 SDK 已实际加载并执行。";\n')
    write("fixtures/module-child.js",'// 模块依赖也由外站地址加载。\nexport const message = "外站模块及其依赖已实际加载。";\n')
    write("fixtures/demo-module.js",'import { message } from "./module-child.js";\nconst state = document.getElementById("module-state"); if (state) state.textContent = message;\n')
    write("fixtures/dynamic.js",'// 动态脚本只修改公开演示状态。\ndocument.documentElement.dataset.labDynamicScript = "loaded";\n')
    write("fixtures/demo.css",f'/* 仅作用于 CSS 样例区域，避免改变应用主题。 */\n@import url("{EXT}/fixtures/imported.css");\n.fixture-css {{ border: 2px dashed var(--accent); padding: 16px; }}\n.fixture-background {{ height: 180px; background: url("{EXT}/fixtures/images/sample.svg") center / contain no-repeat; }}\n')
    write("fixtures/imported.css",'/* 嵌套样式导入样例。 */\n.fixture-imported { font-style: italic; color: var(--accent); }\n')
    fonts=[]
    for ext,format_name in [("woff","woff"),("woff2","woff2"),("ttf","truetype"),("otf","opentype")]:
        fonts.append(f'@font-face {{ font-family: "Lab-{ext}"; src: url("{EXT}/fixtures/fonts/SourceCodePro-Regular.{ext}") format("{format_name}"); font-display: swap; }}\n.fixture-font-{ext} {{ font-family: "Lab-{ext}", monospace; font-size: 20px; }}')
    write("fixtures/fonts/demo-fonts.css","/* 各格式独立加载，仅用于演示段落。 */\n"+'\n'.join(fonts))
    # HLS Packed Audio 以 ID3 PRIV 声明首个采样时间戳，保留原始 MP3 音频帧。
    original=(ROOT/"fixtures/media/t-rex-roar.mp3").read_bytes()
    if original[:3]==b"ID3":
        size=sum((original[6+i]&127) << (7*(3-i)) for i in range(4));original=original[10+size:]
    syncsafe=lambda n:bytes((n>>21&127,n>>14&127,n>>7&127,n&127))
    payload=b"com.apple.streaming.transportStreamTimestamp\0"+bytes(8)
    frame=b"PRIV"+syncsafe(len(payload))+b"\0\0"+payload
    (ROOT/"fixtures/media/roar-hls.mp3").write_bytes(b"ID3\x04\0\0"+syncsafe(len(frame))+frame+original)
    write("fixtures/media/sample.m3u8","#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:3\n#EXT-X-MEDIA-SEQUENCE:0\n#EXTINF:2.116,\nroar-hls.mp3\n#EXT-X-ENDLIST\n")
    for i,(slug,title,intro,priority) in enumerate(CATEGORIES):
        body,head=category_content(slug)
        # IP 页已提供专属暗链与死链，不混入依赖域名的通用外站样例。
        if slug!="ip-address":
            body+=shared_cases(slug,i)
        write(f"links/{slug}/index.html",shell(title,intro,body,slug,head))
    rows=[]
    for slug,title,intro,priority in CATEGORIES:
        count=sum(c["category"]==slug for c in CASES)
        rows.append(f'<li><a class="lab-category" href="/links/{slug}/"><span><strong>{title}</strong><span class="lab-category-description">{intro}</span></span><span class="lab-category-meta"><span class="lab-priority">{priority}</span><span>{count} 个样例 →</span></span></a></li>')
    setup=f'''<details class="lab-setup" open><summary>外站访问准备</summary><p>将 <code>__LAB_EXTERNAL_HOST__</code> 解析到部署主机的 IP。浏览器和扫描器所在设备都需要配置。</p><p>HTTPS 使用项目生成的自签名证书。确认域名解析与证书信任后，可先{link(EXT+'/fixtures/landing.html','访问外站有效目标')}。</p><p>正常样例、暗链和死链都有固定编号。P0 / P1 / P2 是本靶场的检测优先级标记。</p></details>'''
    write("links/index.html",shell("外链检测","选择一个分类，查看真实 HTML 特征、网络请求与预置异常样例。",setup+'<ol class="lab-categories">'+''.join(rows)+'</ol>'))
    write("links/cases.json",json.dumps({"service":"target_web","external_origin":EXT,"categories":[{"slug":s,"title":t,"path":f"/links/{s}/"} for s,t,_,_ in CATEGORIES],"cases":CASES},ensure_ascii=False,indent=2)+"\n")
    print(f"已生成 1 个二级目录页、{len(CATEGORIES)} 个三级页面与 {len(CASES)} 个样例记录。")


if __name__=="__main__":
    generate()
