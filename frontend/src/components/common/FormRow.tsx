import React from "react";
import { Box, Typography } from "@mui/material";

interface FormRowProps {
  label: string;
  children: React.ReactNode;
  labelWidth?: number;
}

const FormRow: React.FC<FormRowProps> = ({
  label,
  children,
  labelWidth = 100,
}) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
    }}
  >
    <Typography
      sx={{
        width: labelWidth,
        flexShrink: 0,
        fontWeight: 600,
      }}
    >
      {label}
    </Typography>
    <Box sx={{ flex: 1 }}>{children}</Box>
  </Box>
);

export default FormRow;
