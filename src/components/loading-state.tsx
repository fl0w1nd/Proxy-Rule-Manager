import { Loader2 } from "lucide-react"

interface LoadingStateProps {
    className?: string
}

export function LoadingState({ className }: LoadingStateProps) {
    return (
        <div className={`flex items-center justify-center py-12 ${className ?? ""}`}>
            <Loader2 className="w-8 h-8 animate-spin text-primary" />
        </div>
    )
}
