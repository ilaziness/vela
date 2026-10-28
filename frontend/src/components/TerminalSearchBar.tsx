import React, { useEffect, useRef, useState } from "react";
import {
  Box,
  IconButton,
  InputBase,
  Tooltip,
  Typography,
} from "@mui/material";
import type { SearchAddon, ISearchOptions } from "@xterm/addon-search";
import CloseIcon from "@mui/icons-material/Close";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import AbcIcon from "@mui/icons-material/Abc";
import CodeIcon from "@mui/icons-material/Code";
import { t } from "../i18n";

const searchDecorations: NonNullable<ISearchOptions["decorations"]> = {
  matchBackground: "#515C6A",
  matchOverviewRuler: "#515C6A",
  activeMatchBackground: "#F5A623",
  activeMatchColorOverviewRuler: "#F5A623",
};

const toggleButtonSx = (active: boolean) => ({
  borderRadius: 1,
  border: 1,
  borderColor: active ? "primary.main" : "divider",
  bgcolor: active ? "action.selected" : "transparent",
  color: active ? "primary.main" : "text.secondary",
  "&:hover": {
    bgcolor: active ? "action.selected" : "action.hover",
  },
});

interface TerminalSearchBarProps {
  open: boolean;
  searchAddon: SearchAddon | null;
  focusNonce: number;
  onClose: () => void;
}

export default function TerminalSearchBar({
  open,
  searchAddon,
  focusNonce,
  onClose,
}: TerminalSearchBarProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [regex, setRegex] = useState(false);
  const [resultIndex, setResultIndex] = useState(-1);
  const [resultCount, setResultCount] = useState(0);
  const [noMatch, setNoMatch] = useState(false);

  const buildOptions = (incremental: boolean): ISearchOptions => ({
    caseSensitive,
    regex,
    incremental,
    decorations: searchDecorations,
  });

  const runFind = (
    term: string,
    direction: "next" | "prev",
    incremental: boolean,
  ) => {
    if (!searchAddon || !term) {
      setResultIndex(-1);
      setResultCount(0);
      setNoMatch(false);
      searchAddon?.clearDecorations();
      return;
    }
    try {
      const found =
        direction === "next"
          ? searchAddon.findNext(term, buildOptions(incremental))
          : searchAddon.findPrevious(term, buildOptions(incremental));
      setNoMatch(!found);
    } catch {
      // Invalid regex (or other SearchAddon errors) must not break the UI.
      setNoMatch(true);
      setResultIndex(-1);
      setResultCount(0);
      searchAddon.clearDecorations();
    }
  };

  useEffect(() => {
    if (!open || !searchAddon) return;
    const disposable = searchAddon.onDidChangeResults((e) => {
      setResultIndex(e.resultIndex);
      setResultCount(e.resultCount);
    });
    return () => {
      disposable.dispose();
    };
  }, [open, searchAddon]);

  useEffect(() => {
    if (!open) return;
    const t = window.setTimeout(() => {
      inputRef.current?.focus();
      inputRef.current?.select();
    }, 0);
    return () => window.clearTimeout(t);
  }, [open, focusNonce]);

  // 输入变化：增量搜索
  useEffect(() => {
    if (!open) return;
    runFind(query, "next", true);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- incremental search on query change
  }, [open, query, searchAddon]);

  // 大小写/正则切换：清缓存后完整重搜，避免 incremental 沿用旧选区导致计数与高亮不一致
  useEffect(() => {
    if (!open) return;
    searchAddon?.clearDecorations();
    runFind(query, "next", false);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- full re-search on option toggle
  }, [caseSensitive, regex]);

  if (!open) {
    return null;
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      onClose();
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      e.stopPropagation();
      runFind(query, e.shiftKey ? "prev" : "next", false);
    }
  };

  let resultLabel = "";
  if (query) {
    if (resultCount > 0 && resultIndex >= 0) {
      resultLabel = `${resultIndex + 1}/${resultCount}`;
    } else if (resultCount > 0) {
      // resultIndex === -1: highlight limit exceeded
      resultLabel = `${resultCount}+`;
    } else {
      resultLabel = "0/0";
    }
  }

  return (
    <Box
      sx={{
        position: "absolute",
        top: 8,
        right: 12,
        zIndex: 10,
        display: "flex",
        alignItems: "center",
        gap: 0.5,
        px: 1,
        py: 0.5,
        borderRadius: 1,
        bgcolor: "background.paper",
        border: 1,
        borderColor: "divider",
        boxShadow: 2,
      }}
      onMouseDown={(e) => e.stopPropagation()}
    >
      <InputBase
        inputRef={inputRef}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder={t("terminal.search_placeholder")}
        sx={{
          ml: 0.5,
          fontSize: 13,
          minWidth: 140,
          "& .MuiInputBase-input": {
            py: 0.5,
            borderBottom: 1,
            borderColor: noMatch && query ? "error.main" : "transparent",
          },
        }}
      />
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ minWidth: 40, textAlign: "center", userSelect: "none" }}
      >
        {resultLabel}
      </Typography>
      <Tooltip title={t("terminal.search_prev")}>
        <IconButton
          size="small"
          onClick={() => runFind(query, "prev", false)}
          disabled={!query}
        >
          <KeyboardArrowUpIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t("terminal.search_next")}>
        <IconButton
          size="small"
          onClick={() => runFind(query, "next", false)}
          disabled={!query}
        >
          <KeyboardArrowDownIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t("terminal.search_case_sensitive")}>
        <IconButton
          size="small"
          aria-pressed={caseSensitive}
          onClick={() => setCaseSensitive((v) => !v)}
          sx={toggleButtonSx(caseSensitive)}
        >
          <AbcIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t("terminal.search_regex")}>
        <IconButton
          size="small"
          aria-pressed={regex}
          onClick={() => setRegex((v) => !v)}
          sx={toggleButtonSx(regex)}
        >
          <CodeIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Tooltip title={t("common.close")}>
        <IconButton size="small" onClick={onClose}>
          <CloseIcon fontSize="small" />
        </IconButton>
      </Tooltip>
    </Box>
  );
}
