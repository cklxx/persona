package persona.feature.assistant

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.test.hasAnyAncestor
import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.annotation.Config
import persona.core.contracts.Session
import persona.core.contracts.SessionKey
import persona.core.contracts.api.*
import persona.core.designsystem.AppStyle
import persona.core.designsystem.PersonaTheme
import persona.core.navigation.AssistantRoute
import persona.core.navigation.TodoDetailRoute

@RunWith(AndroidJUnit4::class)
@Config(sdk = [35], qualifiers = "zh-w400dp-h1500dp")
class AssistantSidebarContentTest {
    @get:Rule val rule = createComposeRule()
    private val operations = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    @After fun close() { operations.cancel(); AssistantManagerChoice.reset() }

    @Test fun historyAndSettingsWorkWithoutRenderingTheAssistantPage() {
        val choice = AssistantManagerChoice.current.value
        val chat = Session(machineId = "example", sessionId = "saved", title = "Saved conversation", assistant = true)
        val initial = AssistantSnapshot(conversations = listOf(chat), manager = choice.manager, managerId = choice.shown)
        var snapshot by mutableStateOf(initial)
        val repository = object : AssistantRepository {
            override val assistant = MutableStateFlow(initial)
            override suspend fun refresh() = Unit
        }
        val hidden = listOf(AssistantRoute, TodoDetailRoute("unsaved-draft"))
        var settings = false
        var opened: SessionKey? = null
        rule.setContent {
            PersonaTheme(style = AppStyle.Brief, reduceMotion = true) {
                Box(Modifier.width(400.dp).height(1200.dp)) {
                    AssistantSidebarContent(snapshot, repository, emptySet(), null, operations, demo = true,
                        onOpen = { opened = it }, onAction = {}, onSettings = { settings = true },
                        onInbox = {}, onKnowledge = {}, onTodos = {}, todoCount = 2, showTodos = true,
                        inboxCount = 3, settingsCount = 0, settingsDot = false, settingsBadge = null,
                        pages = emptyList(), onPage = {}, onTogglePin = {}, onRetired = {})
                }
            }
        }
        rule.onNodeWithText("Saved conversation").assertExists()
        rule.onNodeWithContentDescription("设置").performClick()
        assertTrue(settings)
        assertEquals(listOf(AssistantRoute, TodoDetailRoute("unsaved-draft")), hidden)
        rule.onNodeWithText("Saved conversation").performClick()
        assertEquals(chat.key, opened)
        snapshot = initial.copy(conversations = listOf(chat.copy(title = "Fresh conversation")))
        rule.waitForIdle()
        rule.onNodeWithText("Fresh conversation").assertExists()
        rule.onNodeWithText("Saved conversation").assertDoesNotExist()
    }

    @Test fun archivedConversationRestoresWithoutAssistantHostWhenRefreshFails() {
        val choice = AssistantManagerChoice.current.value
        val archived = Session(machineId = "example", sessionId = "archived", title = "Archived conversation",
            assistant = true, archived = true)
        val snapshot = AssistantSnapshot(conversations = listOf(archived), manager = choice.manager, managerId = choice.shown)
        val restored = mutableListOf<SessionKey>()
        val repository = object : AssistantRepository {
            override val assistant = MutableStateFlow(snapshot)
            override suspend fun refresh() = error("refresh failed")
        }
        val base = persona.feature.assistant.settings.AssistantSettingsManagerTest.Hub(
            mutableMapOf("assistant" to AssistantConfig()))
        val management = object : AssistantManagement by base {
            override fun forManager(manager: String?) = this
            override suspend fun unarchiveConversation(session: SessionKey) { restored += session }
        }
        var opened: SessionKey? = null
        rule.setContent {
            PersonaTheme(style = AppStyle.Brief, reduceMotion = true) {
                Box(Modifier.width(400.dp).height(1200.dp)) {
                    // The production shared sidebar receives management directly; no AssistantAppHost is composed here.
                    AssistantSidebarContent(snapshot, repository, emptySet(), null, operations, demo = false, management = management,
                        onOpen = { opened = it }, onAction = {}, onSettings = {}, onInbox = {}, onKnowledge = {}, onTodos = {},
                        todoCount = 0, showTodos = false, inboxCount = 0, settingsCount = 0, settingsDot = false,
                        settingsBadge = null, pages = emptyList(), onPage = {}, onTogglePin = {}, onRetired = {})
                }
            }
        }
        rule.onNodeWithText("Archived conversation").assertDoesNotExist()
        rule.onNodeWithText("已归档对话").performClick()
        rule.onNode(hasText(archived.title!!) and hasAnyAncestor(hasTestTag("archived-conversations"))).performClick()
        assertEquals(archived.key, opened)
        rule.onNodeWithText("已归档对话").performClick()
        rule.onNodeWithText("恢复").performClick()
        assertEquals(listOf(archived.key), restored)
        rule.onNodeWithText("Archived conversation").assertExists()
        rule.onNodeWithText("已归档对话").assertDoesNotExist()
    }
}
