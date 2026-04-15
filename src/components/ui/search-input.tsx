import { Search } from "lucide-react";
import { cn } from "@/lib/utils";

interface SearchInputProps extends Omit<React.ComponentProps<"input">, "type" | "className"> {
  fullWidth?: boolean;
}

function SearchInput({
  fullWidth = false,
  placeholder = "搜索...",
  ...props
}: SearchInputProps) {
  return (
    <div
      className={cn(
        "flex items-center gap-2 rounded-full border border-border bg-background px-4 py-2 shadow-[var(--shadow-xs)] transition-shadow focus-within:border-primary/40 focus-within:shadow-[var(--shadow-sm)]",
        fullWidth ? "w-full" : "w-full sm:w-auto sm:min-w-[280px]"
      )}
    >
      <Search className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <input
        type="search"
        className="w-full bg-transparent text-sm font-medium text-foreground outline-none placeholder:text-muted-foreground"
        placeholder={placeholder}
        {...props}
      />
    </div>
  );
}

export { SearchInput };
