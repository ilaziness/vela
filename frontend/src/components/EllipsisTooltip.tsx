import { Tooltip, Typography, TypographyProps } from "@mui/material";
import React from "react";

interface EllipsisTooltipProps extends Omit<TypographyProps, "children" | "noWrap"> {
  text: string;
}

/** Single-line ellipsis; hover shows full text. */
const EllipsisTooltip: React.FC<EllipsisTooltipProps> = ({
  text,
  sx,
  ...rest
}) => (
  <Tooltip title={text} enterDelay={400}>
    <Typography noWrap {...rest} sx={{ minWidth: 0, ...sx }}>
      {text}
    </Typography>
  </Tooltip>
);

export default EllipsisTooltip;
