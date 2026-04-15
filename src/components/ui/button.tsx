import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap text-sm font-semibold transition-all duration-150 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none cursor-pointer active:scale-[0.97]",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground rounded-full shadow-[var(--shadow-sm)] hover:scale-[1.03] hover:shadow-[var(--shadow-md)] active:scale-[0.97] focus-visible:ring-[3px] focus-visible:ring-primary/25",
        success:
          "bg-success text-white rounded-full shadow-[var(--shadow-sm)] hover:scale-[1.03] active:scale-[0.97] dark:text-success-foreground",
        destructive:
          "bg-transparent text-destructive rounded-full border border-destructive/30 hover:bg-destructive/8 active:scale-[0.97] focus-visible:ring-[3px] focus-visible:ring-destructive/20",
        outline:
          "bg-background text-foreground rounded-full border border-border shadow-[var(--shadow-xs)] hover:bg-accent hover:border-border-strong active:scale-[0.97] focus-visible:ring-[3px] focus-visible:ring-ring/15",
        secondary:
          "bg-secondary text-secondary-foreground rounded-full border border-border shadow-[var(--shadow-xs)] hover:bg-accent active:scale-[0.97]",
        ghost:
          "rounded-lg hover:bg-accent hover:text-accent-foreground active:scale-[0.97]",
        link: "text-foreground underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-5 py-2",
        sm: "h-8 gap-1.5 px-3 text-xs",
        lg: "h-10 px-6",
        icon: "size-9",
        "icon-sm": "size-8",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
