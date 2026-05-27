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
        "group flex items-center gap-2 rounded-full border border-border bg-background px-4 py-2 shadow-[var(--shadow-xs)] transition-[border-color,box-shadow,background-color] duration-150 ease-out",
        "hover:border-border-strong",
        "focus-within:border-primary/50 focus-within:shadow-[var(--shadow-sm)] focus-within:ring-[3px] focus-within:ring-primary/15",
        fullWidth ? "w-full" : "w-full sm:w-auto sm:min-w-[280px]"
      )}
    >
      <Search
        className="h-4 w-4 shrink-0 text-muted-foreground transition-colors duration-150 group-focus-within:text-foreground"
        aria-hidden="true"
      />
      <input
        type="search"
        className="w-full bg-transparent text-sm font-medium text-foreground outline-none placeholder:text-muted-foreground [&::-webkit-search-cancel-button]:appearance-none"
        placeholder={placeholder}
        {...props}
      />
    </div>
  );
}

export { SearchInput };
