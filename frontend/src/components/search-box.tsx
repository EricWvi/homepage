import { CheckIcon } from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";

import { SiteIcon } from "@/components/site-icon";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useSnapshot } from "@/hooks/use-snapshot";
import { searchEngines, searchUrl } from "@/lib/search-engines";
import { suggest, suggestionDetail } from "@/lib/suggest";
import { cn } from "@/lib/utils";

// Enter on these keeps its own meaning instead of starting a search.
const interactive = "a, button, input, textarea, select, [contenteditable], [role=menuitem]";

/**
 * Always-visible search box. Enter anywhere on the page starts typing,
 * Enter in the box searches in this tab. Every page load starts from Bing;
 * the last engine used is not remembered.
 *
 * While typing, matching sites and site links are listed below. Tab or the
 * arrow keys pick one and Enter opens it; with none picked, Enter searches.
 * Tab switches the engine only while no suggestions are shown.
 */
export function SearchBox() {
  const { snapshot } = useSnapshot();
  const [index, setIndex] = useState(0);
  const [query, setQuery] = useState("");
  // What the suggestions match against. It skips the pinyin an IME shows
  // while composing and catches up once the text is committed.
  const [settled, setSettled] = useState("");
  const [focused, setFocused] = useState(false);
  const [active, setActive] = useState(-1);
  const inputRef = useRef<HTMLInputElement>(null);
  const picked = useRef(false);
  const listId = useId();
  const engine = searchEngines[index]!; // index always wraps within the list

  const suggestions = useMemo(() => suggest(snapshot?.sites ?? [], settled), [snapshot, settled]);
  const icons = useMemo(() => new Map(snapshot?.domains.map((d) => [d.domain, d.icon])), [snapshot]);
  const showList = focused && suggestions.length > 0;

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
    const back = e.key === "ArrowUp" || (e.key === "Tab" && e.shiftKey);
    if (showList && (e.key === "Tab" || e.key === "ArrowDown" || e.key === "ArrowUp")) {
      e.preventDefault();
      const n = suggestions.length;
      // -1 means no pick, so stepping past either end returns to the search.
      setActive((a) => ((back ? a + n + 1 : a + 2) % (n + 1)) - 1);
    } else if (e.key === "Tab") {
      // Without suggestions, Tab switches the engine; with them, only the menu does.
      e.preventDefault();
      setIndex((i) => (i + (back ? -1 : 1) + searchEngines.length) % searchEngines.length);
    } else if (e.key === "Enter") {
      e.preventDefault();
      const pick = showList ? suggestions[active] : undefined;
      if (pick) window.location.assign(pick.url);
      else if (query.trim()) window.location.assign(searchUrl(engine, query.trim()));
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
        role="combobox"
        aria-expanded={showList}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={showList && active >= 0 ? `${listId}-${active}` : undefined}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          if ((e.nativeEvent as InputEvent).isComposing) return;
          setSettled(e.target.value);
          setActive(-1);
        }}
        // Chrome fires the final input event before compositionend, Safari after.
        onCompositionEnd={(e) => {
          setSettled(e.currentTarget.value);
          setActive(-1);
        }}
        onKeyDown={onKeyDown}
        onFocus={() => setFocused(true)}
        onBlur={() => {
          setFocused(false);
          setActive(-1);
        }}
        className="h-12 w-full rounded-full border border-input bg-card pr-5 pl-[3.25rem] text-base shadow-xs transition-[border-color,box-shadow] outline-none placeholder:text-muted-foreground focus:border-ring focus:ring-[3px] focus:ring-ring/30 [&::-webkit-search-cancel-button]:hidden"
      />

      {showList && (
        <ul
          id={listId}
          role="listbox"
          aria-label="网站"
          className="absolute inset-x-0 top-full z-20 mt-2 overflow-hidden rounded-2xl border bg-popover p-1.5 text-popover-foreground shadow-lg"
        >
          {suggestions.map((s, i) => (
            <li key={s.key} id={`${listId}-${i}`} role="option" aria-selected={i === active}>
              <a
                href={s.url}
                tabIndex={-1}
                // Keep focus in the box so the list stays open until the click lands.
                onMouseDown={(e) => e.preventDefault()}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-2.5 py-1.5 hover:bg-accent",
                  i === active && "bg-accent",
                )}
              >
                <SiteIcon
                  domain={s.site.domain}
                  icon={icons.get(s.site.domain)}
                  label={s.site.title}
                  className="size-7 rounded-lg text-[9px] shadow-none"
                />
                <span className="flex min-w-0 flex-col">
                  <span className="truncate text-sm">{s.link?.title ?? s.site.title}</span>
                  <span className="truncate text-xs text-muted-foreground">{suggestionDetail(s)}</span>
                </span>

              </a>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
