import type { ComponentType, SVGProps } from "react";

import {
  BaiduLogo,
  BilibiliLogo,
  BingLogo,
  DoubanLogo,
  GitHubLogo,
  GoogleLogo,
  LdoceLogo,
  TranslateLogo,
  XiaoHongShuLogo,
  ZhihuLogo,
} from "@/components/search-engine-icons";

export type SearchEngine = {
  name: string;
  icon: ComponentType<SVGProps<SVGSVGElement>>;
  /** The encoded query is appended to this URL. */
  pattern: string;
};

/** Ported from the dashboard's ⌘J search. Tab cycles through them in this order. */
export const searchEngines: SearchEngine[] = [
  { name: "必应", icon: BingLogo, pattern: "https://www.bing.com/search?q=" },
  { name: "谷歌", icon: GoogleLogo, pattern: "https://www.google.com/search?q=" },
  { name: "百度", icon: BaiduLogo, pattern: "https://www.baidu.com/s?ie=UTF-8&wd=" },
  { name: "朗文当代", icon: LdoceLogo, pattern: "https://ldoce.onlyquant.top/word/" },
  { name: "翻译", icon: TranslateLogo, pattern: "https://fanyi.baidu.com/#en/zh/" },
  { name: "GitHub", icon: GitHubLogo, pattern: "https://github.com/search?ref=opensearch&q=" },
  {
    name: "小红书",
    icon: XiaoHongShuLogo,
    pattern: "https://www.xiaohongshu.com/search_result?source=web_profile_page&keyword=",
  },
  { name: "哔哩哔哩", icon: BilibiliLogo, pattern: "https://search.bilibili.com/all?from_source=web_search&keyword=" },
  { name: "知乎", icon: ZhihuLogo, pattern: "https://www.zhihu.com/search?type=content&q=" },
  { name: "豆瓣", icon: DoubanLogo, pattern: "https://www.douban.com/search?q=" },
];

export function searchUrl(engine: SearchEngine, query: string): string {
  return engine.pattern + encodeURIComponent(query);
}
