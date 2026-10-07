import { CheckIcon } from "lucide-react";
import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";

import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { searchEngines, searchUrl } from "@/lib/search-engines";
import { cn } from "@/lib/utils";

// Enter on these keeps its own meaning instead of starting a search.
const interactive = "a, button, input, textarea, select, [contenteditable], [role=menuitem]";

/**
 * Always-visible search box. Enter anywhere on the page starts typing,
 * Enter in the box searches in this tab, Tab cycles the engine. Every page
 * load starts from Bing; the last engine used is not remembered.
 */
export function SearchBox() {
  const [index, setIndex] = useState(0);
  const [query, setQuery] = useState("");
  const [focused, setFocused] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const picked = useRef(false);
  const engine = searchEngines[index]!; // index always wraps within the list

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== "Enter" || e.defaultPrevented || e.isComposing) return;
      if (e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return;
      if (e.target instanceof Element && e.target.closest(interactive)) return;
      if (document.querySelector("[role=dialog], [role=alertdialog], [role=menu]")) return;
      e.preventDefault();
      inputRef.current?.focus();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  const onKeyDown = (e: ReactKeyboardEvent<HTMLInputElement>) => {
    // Enter or Tab that confirms an IME candidate belongs to the IME.
    if (e.nativeEvent.isComposing || e.keyCode === 229) return;
    if (e.key === "Tab") {
      e.preventDefault();
      const step = e.shiftKey ? -1 : 1;
      setIndex((i) => (i + step + searchEngines.length) % searchEngines.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const q = query.trim();
      if (q) window.location.assign(searchUrl(engine, q));
    } else if (e.key === "Escape") {
      e.currentTarget.blur();
    }
  };

  return (
    <div className="relative mx-auto mb-16 w-full max-w-[560px]">
      <DropdownMenu>
        <DropdownMenuTrigger
          className="absolute inset-y-0 left-1 flex items-center rounded-full pr-2 pl-4 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          title="切换搜索引擎"
          aria-label={`切换搜索引擎，当前为${engine.name}`}
        >
          <span className="relative size-5">
            {searchEngines.map((e, i) => (
              <e.icon
                key={e.name}
                className={cn(
                  "absolute inset-0 size-5 transition-opacity duration-200",
                  i === index ? "opacity-100" : "opacity-0",
                )}
              />
            ))}
          </span>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="start"
          className="min-w-44"
          onCloseAutoFocus={(e) => {
            // Never hand focus back to the trigger: Enter there would reopen
            // the menu instead of starting a search.
            e.preventDefault();
            if (!picked.current) return;
            picked.current = false;
            inputRef.current?.focus();
          }}
        >
          {searchEngines.map((e, i) => (
            <DropdownMenuItem
              key={e.name}
              onSelect={() => {
                picked.current = true;
                setIndex(i);
              }}
            >
              <e.icon className="size-4" />
              <span className="flex-1">{e.name}</span>
              {i === index && <CheckIcon className="text-muted-foreground" />}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>

      <input
        ref={inputRef}
        type="search"
        aria-label={`使用${engine.name}搜索`}
        placeholder={focused ? `${engine.name}搜索` : "按 Enter 开始搜索"}
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={onKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        className="h-12 w-full rounded-full border border-input bg-card pr-5 pl-[3.25rem] text-base shadow-xs transition-[border-color,box-shadow] outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30 [&::-webkit-search-cancel-button]:hidden"
      />
    </div>
  );
}
