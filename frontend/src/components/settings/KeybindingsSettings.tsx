import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Paper,
  Typography,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  CircularProgress,
} from "@mui/material";
import {
  AppService,
  KeyBindingInfo,
} from "../../../bindings/github.com/ilaziness/vela/services";
import { parseCallServiceError } from "../../func/service";
import { useMessageStore } from "../../stores/message";

const KeybindingsSettings: React.FC = () => {
  const [bindings, setBindings] = useState<KeyBindingInfo[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const list = await AppService.ListKeyBindings();
        if (!cancelled) {
          setBindings(Array.isArray(list) ? list.filter(Boolean) : []);
        }
      } catch (err) {
        if (!cancelled) {
          useMessageStore
            .getState()
            .errorMessage(parseCallServiceError(err) || "加载快捷键失败");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  const groups = useMemo(() => {
    const map = new Map<string, KeyBindingInfo[]>();
    for (const item of bindings) {
      const cat = item.category || "其他";
      const list = map.get(cat) ?? [];
      list.push(item);
      map.set(cat, list);
    }
    return Array.from(map.entries());
  }, [bindings]);

  if (loading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        应用内置快捷键。macOS 使用 ⌘，Windows / Linux 使用 Ctrl。
      </Typography>
      {groups.map(([category, items]) => (
        <Paper key={category} sx={{ p: 2, mb: 2 }} elevation={1}>
          <Typography variant="subtitle1" sx={{ mb: 1, fontWeight: 600 }}>
            {category}
          </Typography>
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>操作</TableCell>
                  <TableCell>快捷键</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.action}>
                    <TableCell>{item.label}</TableCell>
                    <TableCell>{item.keys}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Paper>
      ))}
    </Box>
  );
};

export default KeybindingsSettings;
