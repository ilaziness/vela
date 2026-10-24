declare module '@mui/x-chat-headless/types' {
  interface ChatToolDefinitionMap {
    run_ssh_command: {
      input: { command: string };
      output: { output: string };
    };
    upsert_plan: {
      input: {
        title: string;
        tasks: Array<{ id: string; title: string; status: string }>;
      };
      output: {
        title: string;
        tasks: Array<{ id: string; title: string; status: string }>;
      };
    };
    read_local_file: {
      input: { path: string };
      output: { output: string };
    };
    write_local_file: {
      input: { path: string; content: string };
      output: { output: string };
    };
    list_local_directory: {
      input: { path: string };
      output: { output: string };
    };
    web_search: {
      input: { query: string };
      output: {
        results: Array<{ title: string; url: string; snippet: string }>;
      };
    };
  }
}

export {};
