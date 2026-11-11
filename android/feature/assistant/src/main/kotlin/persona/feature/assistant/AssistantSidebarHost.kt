package persona.feature.assistant

import androidx.compose.runtime.MutableState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import persona.core.contracts.SessionKey

/** Navigation requests consumed when the Assistant home is shown. */
enum class AssistantSidebarAction { Home, NewConversation, MainConversation, ScheduledTasks, SessionInfo, Launcher, Health }

/** The shell owns one history sidebar, independent of the Assistant page's navigation lifetime. */
class AssistantSidebarHost {
    val opened = mutableStateOf(false)
    var personaState by mutableStateOf(persona.core.designsystem.PersonaState.Idle)
    var active by mutableStateOf<persona.core.contracts.SessionKey?>(null)
    var action by mutableStateOf<AssistantSidebarAction?>(null)
    var retired by mutableStateOf(emptySet<SessionKey>())
    private val restoredStates = mutableMapOf<Pair<String?, String>, MutableState<Set<SessionKey>>>()

    /** Successful restores shared by the shell sidebar and the embedded Assistant page, scoped to their assistant. */
    fun restoredState(hubId: String?, managerId: String): MutableState<Set<SessionKey>> =
        restoredStates.getOrPut(hubId to managerId) { mutableStateOf(emptySet()) }
}

val LocalAssistantSidebarHost = staticCompositionLocalOf<AssistantSidebarHost?> { null }

/**
 * The sidebar's head with the assistant switcher, drawn by the shell (docs/assistant/managers.md "Clients"): the head's own
 * padding, the head's other button (the compact sidebar's search), and the head as it was for where no switcher shows
 * (demo mode, a hub without a roster).
 */
typealias AssistantSwitcherSlot = @androidx.compose.runtime.Composable (
    head: androidx.compose.ui.Modifier,
    trailing: (@androidx.compose.runtime.Composable () -> Unit)?,
    fallback: @androidx.compose.runtime.Composable () -> Unit,
) -> Unit
