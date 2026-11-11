package persona.android.ui

import android.app.Application
import android.content.ComponentName
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Shadows
import org.robolectric.annotation.Config
import persona.core.contracts.Session
import persona.core.contracts.SessionKey
import persona.core.contracts.api.AssistantManagerChoice
import persona.core.contracts.api.AssistantSnapshot
import persona.feature.assistant.AssistantSidebarHost
import persona.feature.assistant.LocalAssistantSidebarHost

@RunWith(AndroidJUnit4::class)
@Config(application = Application::class, sdk = [35])
class ShellChatArchiveTest {
    @get:Rule val rule = createEmptyComposeRule()
    @After fun resetManager() { AssistantManagerChoice.reset() }

    private fun launch(content: @Composable () -> Unit): ActivityScenario<ComponentActivity> {
        val app = ApplicationProvider.getApplicationContext<Application>()
        Shadows.shadowOf(app.packageManager).addActivityIfNotPresent(ComponentName(app, ComponentActivity::class.java))
        return ActivityScenario.launch(ComponentActivity::class.java).also { scenario ->
            scenario.onActivity { activity -> activity.setContent(content = content) }
        }
    }

    @Test fun onlyTheOpenArchivedAssistantIsHistoryAndRestoreClearsThatState() {
        val archived = Session(machineId = "pro~g0a1b2c3d", sessionId = "history", title = "Archived", archived = true)
        val snapshot = AssistantSnapshot(conversations = listOf(archived))
        assertEquals(archived, archivedHistorySession(snapshot, archived.key, assistant = true))
        assertNull(archivedHistorySession(snapshot, archived.key, assistant = false))
        assertNull(archivedHistorySession(snapshot, SessionKey(archived.machineId, "other"), assistant = true))
        val restored = snapshot.copy(conversations = snapshot.conversations.map { it.copy(archived = false) })
        assertNull(archivedHistorySession(restored, archived.key, assistant = true))
    }

    @Test fun standaloneChatUsesSharedRestoreAcrossManagerSwitches() {
        val archived = Session(machineId = "hub", sessionId = "history", title = "Archived", archived = true)
        val aSnapshot = AssistantSnapshot(conversations = listOf(archived), managerId = "a")
        val bSnapshot = AssistantSnapshot(conversations = listOf(archived.copy(archived = false)), manager = "b", managerId = "b")
        val host = AssistantSidebarHost()
        var snapshot by mutableStateOf(aSnapshot)
        var resolved: Session? = null
        AssistantManagerChoice.choose(null, null, "a")
        val scenario = launch {
            CompositionLocalProvider(LocalAssistantSidebarHost provides host) {
                resolved = rememberArchivedHistorySession(snapshot, archived.key, assistant = true)
            }
        }
        rule.waitForIdle()
        assertEquals(archived, resolved)

        rule.runOnIdle {
            AssistantManagerChoice.pick("b")
            snapshot = bSnapshot.copy(conversations = emptyList())
        }
        rule.waitForIdle()
        assertEquals(archived, resolved)
        assertTrue(host.restoredState(null, "a").value.isEmpty())

        rule.runOnIdle {
            AssistantManagerChoice.pick(null)
            snapshot = aSnapshot
        }
        rule.waitForIdle()
        assertEquals(archived, resolved)

        rule.runOnIdle { host.restoredState(null, "a").value = setOf(archived.key) }
        rule.waitForIdle()
        assertNull(resolved)

        rule.runOnIdle {
            AssistantManagerChoice.pick("b")
            snapshot = bSnapshot
        }
        rule.waitForIdle()
        assertNull(resolved)
        assertTrue(archived.key in host.restoredState(null, "a").value)

        rule.runOnIdle {
            AssistantManagerChoice.pick(null)
            snapshot = aSnapshot
        }
        rule.waitForIdle()
        assertNull(resolved)
        assertTrue(archived.key in host.restoredState(null, "a").value)

        rule.runOnIdle {
            snapshot = aSnapshot.copy(conversations = listOf(archived.copy(archived = false)))
        }
        rule.waitForIdle()
        assertTrue(host.restoredState(null, "a").value.isEmpty())
        assertNull(resolved)
        scenario.close()
    }
}
