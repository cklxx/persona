package persona.android.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test
import persona.core.contracts.Session
import persona.core.contracts.SessionKey
import persona.core.contracts.api.AssistantSnapshot

class ShellChatArchiveTest {
    @Test fun onlyTheOpenArchivedAssistantIsHistoryAndRestoreClearsThatState() {
        val archived = Session(machineId = "pro~g0a1b2c3d", sessionId = "history", title = "Archived", archived = true)
        val snapshot = AssistantSnapshot(conversations = listOf(archived))
        assertEquals(archived, archivedHistorySession(snapshot, archived.key, assistant = true))
        assertNull(archivedHistorySession(snapshot, archived.key, assistant = false))
        assertNull(archivedHistorySession(snapshot, SessionKey(archived.machineId, "other"), assistant = true))
        val restored = snapshot.copy(conversations = snapshot.conversations.map { it.copy(archived = false) })
        assertNull(archivedHistorySession(restored, archived.key, assistant = true))
    }
}
